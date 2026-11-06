package purelymail

// ListDomainsRequest lists domains accessible to the account.
type ListDomainsRequest struct {
	IncludeShared *bool `json:"includeShared,omitempty"`
}

// ApiDomainDnsSummary summarizes the domain's DNS health checks.
type ApiDomainDnsSummary struct {
	PassesMx    *bool `json:"passesMx"`
	PassesSpf   *bool `json:"passesSpf"`
	PassesDkim  *bool `json:"passesDkim"`
	PassesDmarc *bool `json:"passesDmarc"`
}

// ApiDomainInfo describes a domain under the account.
type ApiDomainInfo struct {
	Name                  string               `json:"name"`
	AllowAccountReset     *bool                `json:"allowAccountReset"`
	SymbolicSubaddressing *bool                `json:"symbolicSubaddressing"`
	IsShared              *bool                `json:"isShared"`
	DnsSummary            *ApiDomainDnsSummary `json:"dnsSummary"`
}

// CreateUserRequest creates a user under the account's domain.
type CreateUserRequest struct {
	UserName                 string `json:"userName"`
	DomainName               string `json:"domainName"`
	Password                 string `json:"password"`
	EnablePasswordReset      *bool  `json:"enablePasswordReset,omitempty"`
	RecoveryEmail            string `json:"recoveryEmail,omitempty"`
	RecoveryEmailDescription string `json:"recoveryEmailDescription,omitempty"`
	RecoveryPhone            string `json:"recoveryPhone,omitempty"`
	RecoveryPhoneDescription string `json:"recoveryPhoneDescription,omitempty"`
	EnableSearchIndexing     *bool  `json:"enableSearchIndexing,omitempty"`
	SendWelcomeEmail         *bool  `json:"sendWelcomeEmail,omitempty"`
}

// GetUserRequest fetches a single user's settings.
type GetUserRequest struct {
	UserName string `json:"userName"`
}

// GetUserResponse holds a user's settings.
type GetUserResponse struct {
	EnableSearchIndexing           *bool `json:"enableSearchIndexing"`
	RecoveryEnabled                *bool `json:"recoveryEnabled"`
	RequireTwoFactorAuthentication *bool `json:"requireTwoFactorAuthentication"`
	EnableSpamFiltering            *bool `json:"enableSpamFiltering"`
}

// ModifyUserRequest modifies a user's settings.
type ModifyUserRequest struct {
	UserName                       string `json:"userName"`
	NewUserName                    string `json:"newUserName,omitempty"`
	NewPassword                    string `json:"newPassword,omitempty"`
	EnableSearchIndexing           *bool  `json:"enableSearchIndexing,omitempty"`
	EnablePasswordReset            *bool  `json:"enablePasswordReset,omitempty"`
	RequireTwoFactorAuthentication *bool  `json:"requireTwoFactorAuthentication,omitempty"`
}

// DeleteUserRequest deletes a user (full username, e.g. user@domain.com).
type DeleteUserRequest struct {
	UserName string `json:"userName"`
}
