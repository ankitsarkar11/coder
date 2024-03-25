package spice

import (
	"context"
	"database/sql"

	"github.com/authzed/authzed-go/pkg/requestmeta"
	"github.com/authzed/authzed-go/pkg/responsemeta"
	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/spicedb/pkg/cmd/server"
	"github.com/authzed/spicedb/pkg/tuple"
	"github.com/google/uuid"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"tailscale.com/syncs"

	"cdr.dev/slog"
	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/database/spice/debug"
	"github.com/coder/coder/v2/coderd/database/spice/policy"
	playground "github.com/coder/coder/v2/coderd/database/spice/policy/playground/relationships"
)

const (
	wrapname = "spicedb.querier"
	god      = "god"
)

type spiceActorKey struct{}

// NoActorError wraps ErrNoRows for the api to return a 404. This is the correct
// response when the user is not authorized.
var NoActorError = xerrors.Errorf("no authorization actor in context: %w", sql.ErrNoRows)

func ActorFromContext(ctx context.Context) (*v1.SubjectReference, bool) {
	a, ok := ctx.Value(spiceActorKey{}).(*v1.SubjectReference)
	return a, ok
}

// AsGod is a hack to get around bootstrapping. Since we need perms to create the
// first users. Idk if we should keep this.
func AsGod(ctx context.Context) context.Context {
	return context.WithValue(ctx, spiceActorKey{}, &v1.SubjectReference{
		Object: &v1.ObjectReference{
			ObjectType: god,
			ObjectId:   god,
		},
	})
}

func AsUser(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, spiceActorKey{}, &v1.SubjectReference{
		Object: policy.New().User(userID).Object(),
	})
}

type SpiceDB struct {
	// TODO: Do not embed anonymously. This is just a lazy way to skip
	// 		having to implement all interface methods for now.
	database.Store
	// reverts is only used if in a transaction.
	reverts reverter
	logger  slog.Logger

	srv       server.RunnableServer
	schemaCli v1.SchemaServiceClient
	permCli   v1.PermissionsServiceClient
	// experimental client has bulk operations.
	expCli v1.ExperimentalServiceClient

	// zedToken is required to enforce consistency. When making a request, passing
	// this token says "I want to see the world as if it was at least after this time".
	// For a 100% consistent view, we should update this on any write.
	// In a world of HA, we have an issue that a different Coder might have done
	// a write.
	// TODO: A way of doing this across HA is storing the Zedtoken in the DB on
	//		each resource. So if you fetch a workspace, it has the Zedtoken required
	//		to get it's updated state.
	zedToken syncs.AtomicValue[*v1.ZedToken]

	ctx    context.Context
	cancel context.CancelFunc

	// debug will print extra debug information on all checks.
	debug bool
}

func (s *SpiceDB) Wrappers() []string {
	return append(s.Store.Wrappers(), wrapname)
}

// TODO: Should we do this automatically on New()?
func (s *SpiceDB) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	s.ctx = ctx
	s.cancel = cancel

	// Start the server
	go func() {
		if err := s.srv.Run(ctx); err != nil {
			s.logger.Error(ctx, "spicedb run server", slog.Error(err))
		}
	}()

	// Setup the clients
	conn, err := s.srv.GRPCDialContext(ctx)
	if err != nil {
		return xerrors.Errorf("spicedb grpc dial failed: %w", err)
	}

	s.expCli = v1.NewExperimentalServiceClient(conn)
	s.schemaCli = v1.NewSchemaServiceClient(conn)
	s.permCli = v1.NewPermissionsServiceClient(conn)

	// TODO: If the server isn't running yet because it's async, will this fail?
	resp, err := s.schemaCli.WriteSchema(ctx, &v1.WriteSchemaRequest{
		Schema: policy.Schema,
	})
	if err != nil {
		return xerrors.Errorf("write schema: %w", err)
	}
	s.zedToken.Store(resp.WrittenAt)

	return nil
}

func (s *SpiceDB) Debugging(set bool) {
	s.debug = set
}

func (s *SpiceDB) Close() {
	s.cancel()
}

// WithRelationsExec allows exec functions that do not return a return object.
func WithRelationsExec[A any](ctx context.Context, s *SpiceDB, relations []v1.Relationship, do func(ctx context.Context, arg A) error, arg A) error {
	_, err := WithRelations(ctx, s, relations, func(ctx context.Context, arg A) (interface{}, error) {
		return nil, do(ctx, arg)
	}, arg)
	return err
}

func WithRelations[A any, R any](ctx context.Context, s *SpiceDB, relations []v1.Relationship, do func(ctx context.Context, arg A) (R, error), arg A) (R, error) {
	var empty R
	revert, err := s.WriteRelationships(ctx, relations...)
	if err != nil {
		return empty, xerrors.Errorf("write relationships: %w", err)
	}
	r, err := do(ctx, arg)
	if err != nil {
		revert()
		return r, err
	}
	return r, nil
}

func (s *SpiceDB) WithRelations(ctx context.Context, relations []v1.Relationship, do func() error) (context.Context, error) {
	revert, err := s.WriteRelationships(ctx, relations...)
	if err != nil {
		return nil, xerrors.Errorf("write relationships: %w", err)
	}

	ctx = context.WithValue(ctx, "revert", revert)
	return ctx, nil
}

// WriteRelationships returns a revert function that will delete all the relationships that
// were written.
func (s *SpiceDB) WriteRelationships(ctx context.Context, relationships ...v1.Relationship) (revert func(), _ error) {
	opts := []grpc.CallOption{}
	if s.debugging(ctx) {
		wlogger := s.logger.With(slog.F("write_relationships", playground.RelationshipsToStrings(relationships)))
		debugCtx, opt, callback := debugSpiceDBRPC(ctx, wlogger)
		opts = append(opts, opt)
		defer callback()
		ctx = debugCtx
	}

	updates := make([]*v1.RelationshipUpdate, 0, len(relationships))
	for i := range relationships {
		// Make a copy so to ensure the delete function has the correct data.
		// We could definitely improve the memory allocations here.
		cpy := relationships[i]
		updates = append(updates, &v1.RelationshipUpdate{
			Operation:    v1.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: &cpy,
		})
	}

	// A relationship can be written like this:
	//	group:hr#member@user:camilla
	// And parsed with:
	// 	tup := tuple.Parse(rel)
	// 	v1Rel := tuple.ToRelationship(tup)
	resp, err := s.permCli.WriteRelationships(ctx, &v1.WriteRelationshipsRequest{
		Updates:               updates,
		OptionalPreconditions: nil,
	}, opts...)
	if err != nil {
		return nil, xerrors.Errorf("write relationship: %w", err)
	}
	// TODO: We should probably return this? Allow it to be stored on the object or something?
	s.zedToken.Store(resp.WrittenAt)

	// revert is an optional callback the caller can use to delete the relationship
	// if it's no longer needed. This is helpful if their tx fails.
	revert = func() {
		for i := range updates {
			updates[i].Operation = v1.RelationshipUpdate_OPERATION_DELETE
		}

		// The delete api might be quicker, but this an atomic operation.
		resp, err := s.permCli.WriteRelationships(ctx, &v1.WriteRelationshipsRequest{
			Updates:               updates,
			OptionalPreconditions: nil,
		}, opts...)
		if resp != nil && resp.WrittenAt != nil {
			s.zedToken.Store(resp.WrittenAt)
		}
		if err != nil {
			// Log out all the relationships that might have failed.
			rels := make([]string, 0, len(updates))
			for _, up := range updates {
				str, _ := tuple.StringRelationship(up.Relationship)
				rels = append(rels, str)
			}
			s.logger.Error(ctx, "revert relationships",
				slog.Error(err),
				slog.F("quantity", len(updates)),
				slog.F("relationships", rels),
			)
		}
	}

	// If we are in a tx, handle all reverts as a single batch.
	// This will make sure any single failure triggers every revert
	// in the same tx.
	if s.reverts != nil {
		s.reverts.AddRevert(revert)
		revert = noop
	}
	return revert, nil
}

func (s *SpiceDB) Check(ctx context.Context, permission string, resource *v1.ObjectReference) error {
	actor, ok := ActorFromContext(ctx)
	if !ok {
		return NoActorError
	}

	if actor.Object.ObjectType == god && actor.Object.ObjectId == "god" {
		return nil
	}

	opts := []grpc.CallOption{}
	if s.debugging(ctx) {
		debugCtx, opt, callback := debugSpiceDBRPC(ctx, s.logger)
		opts = append(opts, opt)
		defer callback()
		ctx = debugCtx
	}

	// A permission can be written like:
	//	"<object_type:object_id>#<permission>@<subject_type:subject_id>"
	//	"workspace:dogfood#view@user:root"
	// And parsed with:
	//	tup := tuple.Parse(perm)
	//	r := tuple.ToRelationship(tup)
	resp, err := s.permCli.CheckPermission(ctx, &v1.CheckPermissionRequest{
		Consistency: &v1.Consistency{Requirement: &v1.Consistency_AtLeastAsFresh{s.zedToken.Load()}},
		Resource:    resource,
		Permission:  permission,
		Subject:     actor,
		// Context for caveats
		Context: nil,
	}, opts...)
	if err != nil {
		return xerrors.Errorf("check permission: %w", err)
	}

	if resp.Permissionship == v1.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION {
		return nil
	}
	if resp.Permissionship == v1.CheckPermissionResponse_PERMISSIONSHIP_CONDITIONAL_PERMISSION {
		return xerrors.Errorf("not authorized: conditional permission")
	}
	return xerrors.Errorf("not authorized")
}

//func (s *SpiceDB) Lookup(ctx context.Context, permission string, resource *v1.ObjectReference) ([]uuid.UUID, error) {

//}

func debugSpiceDBRPC(ctx context.Context, logger slog.Logger) (debugCtx context.Context, opt grpc.CallOption, debugString func()) {
	var trailerMD metadata.MD
	ctx = requestmeta.AddRequestHeaders(ctx, requestmeta.RequestDebugInformation)
	debugString = func() {
		if trailerMD.Len() == 0 {
			return
		}

		fields := []any{} // The only way to make the compiler happy
		if count, err := responsemeta.GetIntResponseTrailerMetadata(trailerMD, responsemeta.CachedOperationsCount); err == nil {
			// The number of cached operations hit.
			fields = append(fields, slog.F("cached_operations_count", count))
		}
		if count, err := responsemeta.GetIntResponseTrailerMetadata(trailerMD, responsemeta.DispatchedOperationsCount); err == nil {
			// The number of dispatched operations
			fields = append(fields, slog.F("dispatched_operations_count", count))
		}

		msg := "debug rpc"
		// This debug information key should be present for PermissionChecks. It
		// is not present in all responses (like write responses)
		found, err := responsemeta.GetResponseTrailerMetadata(trailerMD, responsemeta.DebugInformation)
		if err == nil {
			debugInfo := &v1.DebugInformation{}
			err = protojson.Unmarshal([]byte(found), debugInfo)
			if err != nil {
				logger.Debug(ctx, "debug rpc failed: unable to debug proto", slog.Error(err))
				return
			}

			if debugInfo.Check == nil {
				logger.Debug(ctx, "debug rpc: no trace found for the check")
				return
			}
			tp := debug.NewTreePrinter()
			debug.DisplayCheckTrace(debugInfo.Check, tp, false)
			msg = tp.String()
		}

		logger.Debug(ctx, msg, fields...)
	}

	return ctx, grpc.Trailer(&trailerMD), debugString
}