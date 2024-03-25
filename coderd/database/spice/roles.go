package spice

import (
	"context"
	"errors"
	"fmt"
	"io"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/google/uuid"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"

	"github.com/coder/coder/v2/coderd/database/spice/policy"
)

// OrganizationRoleAssignedActors returns the actors (users/groups) assigned to the
// given role in the given organization.
// TODO: the name of this method is very confusing.
func (s *SpiceDB) OrganizationRoleAssignedActors(ctx context.Context, roleName string) ([]string, error) {
	opts := []grpc.CallOption{}
	if s.debugging(ctx) {
		debugCtx, opt, callback := debugSpiceDBRPC(ctx, s.logger)
		opts = append(opts, opt)
		defer callback()
		ctx = debugCtx
	}

	builder := policy.New()
	role := builder.Org_role(policy.String(roleName))
	resp, err := s.permCli.ReadRelationships(context.Background(), &v1.ReadRelationshipsRequest{
		Consistency: &v1.Consistency{Requirement: &v1.Consistency_AtLeastAsFresh{s.zedToken.Load()}},
		RelationshipFilter: &v1.RelationshipFilter{
			ResourceType:             role.Object().ObjectType,
			OptionalResourceId:       role.Object().ObjectId,
			OptionalResourceIdPrefix: "",
			OptionalRelation:         role.RelationMember(),
			OptionalSubjectFilter:    nil,
		},
		OptionalLimit:  0,
		OptionalCursor: nil,
	}, opts...)
	if err != nil {
		return nil, err
	}

	assigned := make([]string, 0)
	for true {
		rel, err := resp.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, xerrors.Errorf("failed to find assigned: %w", err)
		}
		// rel.Relationship.Subject.Object.ObjectType is either Group or User.
		// We might want to return that info as well? Will be helpful for the caller.
		assigned = append(assigned, rel.Relationship.Subject.Object.ObjectId)
	}
	return assigned, nil
}

// OrganizationRolePermissions returns the permissions assigned to the given role in the given organization.
// These strings refer to the relation names on the 'definition Organization' object.
func (s *SpiceDB) OrganizationRolePermissions(ctx context.Context, roleName string, orgID uuid.UUID) ([]string, error) {
	opts := []grpc.CallOption{}
	if s.debugging(ctx) {
		debugCtx, opt, callback := debugSpiceDBRPC(ctx, s.logger)
		opts = append(opts, opt)
		defer callback()
		ctx = debugCtx
	}

	resp, err := s.permCli.ReadRelationships(context.Background(), &v1.ReadRelationshipsRequest{
		Consistency:        &v1.Consistency{Requirement: &v1.Consistency_AtLeastAsFresh{s.zedToken.Load()}},
		RelationshipFilter: orgRolePermissionsFilter(roleName, orgID),
		OptionalLimit:      0,
		OptionalCursor:     nil,
	}, opts...)
	if err != nil {
		return nil, err
	}

	perms := make([]string, 0)
	for true {
		rel, err := resp.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, xerrors.Errorf("failed to read relationship: %w", err)
		}
		perms = append(perms, rel.Relationship.Relation)
	}
	return perms, nil
}

// UpsertCustomOrganizationRole creates or updates a custom organization role with the given name.
// If the role already exists, calling this function will remove all existing permissions
// and replace them with the ones handled by the `assignPerms` function.
// This means to remove all permissions, you can call this function with an empty `assignPerms` function.
//
// Any assigned users will remain.
//
// TODO: Name conflicts of 2 orgs sharing the same role name? We should probably append the org id to the role
// name to have this never happen. It would break perms.
func (s *SpiceDB) UpsertCustomOrganizationRole(ctx context.Context, roleName string, orgID uuid.UUID, assignPerms func(role *policy.ObjOrg_role, organization *policy.ObjOrganization)) error {
	// TODO: Do a perm check if the user can make a custom org role?

	builder := policy.New()
	org := builder.Organization(orgID)
	newRole := builder.Org_role(policy.String(roleName))
	// Associate to the specific org
	// TODO: I think this just replaces the existing relation each time.
	// 	Should we check if it already exists and skip if it does?
	newRole.Organization(org)
	// Let the caller assign permissions.
	assignPerms(newRole, org)

	// Remove all existing permissions for this role for the upsert.
	// This ensures a clean slate, so all perms must be provided in the
	// function call.
	resp, err := s.permCli.DeleteRelationships(ctx, &v1.DeleteRelationshipsRequest{
		RelationshipFilter:            orgRolePermissionsFilter(roleName, orgID),
		OptionalPreconditions:         nil,
		OptionalLimit:                 0,
		OptionalAllowPartialDeletions: false,
	})
	if err != nil {
		return fmt.Errorf("failed to delete existing relationships: %w", err)
	}
	s.zedToken.Store(resp.DeletedAt)

	// Add the new permissions.
	_, err = s.WriteRelationships(ctx, builder.Relationships...)
	if err != nil {
		return fmt.Errorf("failed to write relationships: %w", err)
	}

	// TODO: Write to coderd database with the new roles?

	return nil
}

func orgRolePermissionsFilter(roleName string, orgID uuid.UUID) *v1.RelationshipFilter {
	builder := policy.New()
	role := builder.Org_role(policy.String(roleName)).AsAnyHas_role().AsSubject()
	org := builder.Organization(orgID).Object()
	return &v1.RelationshipFilter{
		// Remove all relations from the org to this role.
		// This will strip it of all it's permissions, and leave a clean slate.
		// Any assigned users will remain though!
		ResourceType:             org.ObjectType,
		OptionalResourceId:       org.ObjectId,
		OptionalResourceIdPrefix: "",
		OptionalRelation:         "",
		// The role is the subject in these relations. This filter will find
		// all relations from the org to the role.
		OptionalSubjectFilter: &v1.SubjectFilter{
			SubjectType:       role.Object.ObjectType,
			OptionalSubjectId: role.Object.ObjectId,
			OptionalRelation: &v1.SubjectFilter_RelationFilter{
				Relation: role.OptionalRelation,
			},
		},
	}
}