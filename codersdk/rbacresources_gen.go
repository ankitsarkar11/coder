// Code generated by rbacgen/main.go. DO NOT EDIT.
package codersdk

type RBACResource string

const (
	ResourceWildcard           RBACResource = "*"
	ResourceApiKey             RBACResource = "api_key"
	ResourceAssignOrgRole      RBACResource = "assign_org_role"
	ResourceAssignRole         RBACResource = "assign_role"
	ResourceAuditLog           RBACResource = "audit_log"
	ResourceDebugInfo          RBACResource = "debug_info"
	ResourceDeploymentConfig   RBACResource = "deployment_config"
	ResourceDeploymentStats    RBACResource = "deployment_stats"
	ResourceFile               RBACResource = "file"
	ResourceFrobulator         RBACResource = "frobulator"
	ResourceGroup              RBACResource = "group"
	ResourceLicense            RBACResource = "license"
	ResourceOauth2App          RBACResource = "oauth2_app"
	ResourceOauth2AppCodeToken RBACResource = "oauth2_app_code_token"
	ResourceOauth2AppSecret    RBACResource = "oauth2_app_secret"
	ResourceOrganization       RBACResource = "organization"
	ResourceOrganizationMember RBACResource = "organization_member"
	ResourceProvisionerDaemon  RBACResource = "provisioner_daemon"
	ResourceProvisionerKeys    RBACResource = "provisioner_keys"
	ResourceReplicas           RBACResource = "replicas"
	ResourceSystem             RBACResource = "system"
	ResourceTailnetCoordinator RBACResource = "tailnet_coordinator"
	ResourceTemplate           RBACResource = "template"
	ResourceUser               RBACResource = "user"
	ResourceWorkspace          RBACResource = "workspace"
	ResourceWorkspaceDormant   RBACResource = "workspace_dormant"
	ResourceWorkspaceProxy     RBACResource = "workspace_proxy"
)

type RBACAction string

const (
	ActionApplicationConnect RBACAction = "application_connect"
	ActionAssign             RBACAction = "assign"
	ActionCreate             RBACAction = "create"
	ActionDelete             RBACAction = "delete"
	ActionRead               RBACAction = "read"
	ActionReadPersonal       RBACAction = "read_personal"
	ActionSSH                RBACAction = "ssh"
	ActionUpdate             RBACAction = "update"
	ActionUpdatePersonal     RBACAction = "update_personal"
	ActionUse                RBACAction = "use"
	ActionViewInsights       RBACAction = "view_insights"
	ActionWorkspaceStart     RBACAction = "start"
	ActionWorkspaceStop      RBACAction = "stop"
)

// RBACResourceActions is the mapping of resources to which actions are valid for
// said resource type.
var RBACResourceActions = map[RBACResource][]RBACAction{
	ResourceWildcard:           {},
	ResourceApiKey:             {ActionCreate, ActionDelete, ActionRead, ActionUpdate},
	ResourceAssignOrgRole:      {ActionAssign, ActionCreate, ActionDelete, ActionRead},
	ResourceAssignRole:         {ActionAssign, ActionCreate, ActionDelete, ActionRead},
	ResourceAuditLog:           {ActionCreate, ActionRead},
	ResourceDebugInfo:          {ActionRead},
	ResourceDeploymentConfig:   {ActionRead, ActionUpdate},
	ResourceDeploymentStats:    {ActionRead},
	ResourceFile:               {ActionCreate, ActionRead},
	ResourceFrobulator:         {ActionCreate, ActionDelete, ActionRead, ActionUpdate},
	ResourceGroup:              {ActionCreate, ActionDelete, ActionRead, ActionUpdate},
	ResourceLicense:            {ActionCreate, ActionDelete, ActionRead},
	ResourceOauth2App:          {ActionCreate, ActionDelete, ActionRead, ActionUpdate},
	ResourceOauth2AppCodeToken: {ActionCreate, ActionDelete, ActionRead},
	ResourceOauth2AppSecret:    {ActionCreate, ActionDelete, ActionRead, ActionUpdate},
	ResourceOrganization:       {ActionCreate, ActionDelete, ActionRead, ActionUpdate},
	ResourceOrganizationMember: {ActionCreate, ActionDelete, ActionRead, ActionUpdate},
	ResourceProvisionerDaemon:  {ActionCreate, ActionDelete, ActionRead, ActionUpdate},
	ResourceProvisionerKeys:    {ActionCreate, ActionDelete, ActionRead},
	ResourceReplicas:           {ActionRead},
	ResourceSystem:             {ActionCreate, ActionDelete, ActionRead, ActionUpdate},
	ResourceTailnetCoordinator: {ActionCreate, ActionDelete, ActionRead, ActionUpdate},
	ResourceTemplate:           {ActionCreate, ActionDelete, ActionRead, ActionUpdate, ActionViewInsights},
	ResourceUser:               {ActionCreate, ActionDelete, ActionRead, ActionReadPersonal, ActionUpdate, ActionUpdatePersonal},
	ResourceWorkspace:          {ActionApplicationConnect, ActionCreate, ActionDelete, ActionRead, ActionSSH, ActionWorkspaceStart, ActionWorkspaceStop, ActionUpdate},
	ResourceWorkspaceDormant:   {ActionApplicationConnect, ActionCreate, ActionDelete, ActionRead, ActionSSH, ActionWorkspaceStart, ActionWorkspaceStop, ActionUpdate},
	ResourceWorkspaceProxy:     {ActionCreate, ActionDelete, ActionRead, ActionUpdate},
}
