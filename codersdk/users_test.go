package codersdk_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/coder/coder/v2/codersdk"
)

func TestDeprecatedCreateUserRequest(t *testing.T) {
	t.Parallel()

	t.Run("DefaultOrganization", func(t *testing.T) {
		t.Parallel()

		input := `
{
   "email":"alice@coder.com",
   "password":"hunter2",
   "username":"alice",
   "name":"alice",
   "organization_id":"00000000-0000-0000-0000-000000000000",
   "disable_login":false,
   "login_type":"none"
}
`
		var req codersdk.CreateUserRequest
		err := json.Unmarshal([]byte(input), &req)
		require.NoError(t, err)
		require.Equal(t, req.Email, "alice@coder.com")
		require.Equal(t, req.Password, "hunter2")
		require.Equal(t, req.Username, "alice")
		require.Equal(t, req.Name, "alice")
		require.Equal(t, req.OrganizationIDs, []uuid.UUID{uuid.Nil})
		require.Equal(t, req.UserLoginType, codersdk.LoginTypeNone)
	})

	t.Run("MultipleOrganizations", func(t *testing.T) {
		t.Parallel()

		input := `
{
   "email":"alice@coder.com",
   "password":"hunter2",
   "username":"alice",
   "name":"alice",
   "organization_id":"00000000-0000-0000-0000-000000000000",
   "organization_ids":["a618cb03-99fb-4380-adb6-aa801629a4cf","8309b0dc-44ea-435d-a9ff-72cb302835e4"],
   "disable_login":false,
   "login_type":"none"
}
`
		var req codersdk.CreateUserRequest
		err := json.Unmarshal([]byte(input), &req)
		require.NoError(t, err)
		require.Equal(t, req.Email, "alice@coder.com")
		require.Equal(t, req.Password, "hunter2")
		require.Equal(t, req.Username, "alice")
		require.Equal(t, req.Name, "alice")
		require.ElementsMatch(t, req.OrganizationIDs,
			[]uuid.UUID{
				uuid.Nil,
				uuid.MustParse("a618cb03-99fb-4380-adb6-aa801629a4cf"),
				uuid.MustParse("8309b0dc-44ea-435d-a9ff-72cb302835e4"),
			})

		require.Equal(t, req.UserLoginType, codersdk.LoginTypeNone)
	})

	t.Run("OmittedOrganizations", func(t *testing.T) {
		t.Parallel()

		input := `
{
   "email":"alice@coder.com",
   "password":"hunter2",
   "username":"alice",
   "name":"alice",
   "disable_login":false,
   "login_type":"none"
}
`
		var req codersdk.CreateUserRequest
		err := json.Unmarshal([]byte(input), &req)
		require.NoError(t, err)

		require.Empty(t, req.OrganizationIDs)
	})
}
