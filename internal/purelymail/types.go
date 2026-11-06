// Package purelymail ... (see package doc in client.go)
package purelymail

import "fmt"

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

// AddDomainRequest adds a domain to the account.
type AddDomainRequest struct {
	DomainName string `json:"domainName"`
}

// GetOwnershipCodeRequest fetches the TXT ownership-proof code for a domain.
type GetOwnershipCodeRequest struct {
	DomainName string `json:"domainName"`
}

// GetOwnershipCodeResponse returns the TXT ownership-proof code.
type GetOwnershipCodeResponse struct {
	Code string `json:"code"`
}

// UpdateDomainSettingsRequest updates settings for a domain.
type UpdateDomainSettingsRequest struct {
	Name                  string `json:"name"`
	AllowAccountReset     *bool  `json:"allowAccountReset,omitempty"`
	SymbolicSubaddressing *bool  `json:"symbolicSubaddressing,omitempty"`
	RecheckDNS            *bool  `json:"recheckDns,omitempty"`
}

// DeleteDomainRequest deletes a domain.
type DeleteDomainRequest struct {
	Name string `json:"name"`
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

// CreateRoutingRuleRequest creates a routing rule for a domain.
type CreateRoutingRuleRequest struct {
	DomainName      string   `json:"domainName"`
	Prefix          bool     `json:"prefix"`
	MatchUser       string   `json:"matchUser"`
	TargetAddresses []string `json:"targetAddresses"`
	Catchall        bool     `json:"catchall"`
}

// RoutingRule is a routing/catch-all rule under the account.
type RoutingRule struct {
	ID              int64    `json:"id"`
	DomainName      string   `json:"domainName"`
	Prefix          bool     `json:"prefix"`
	MatchUser       string   `json:"matchUser"`
	TargetAddresses []string `json:"targetAddresses"`
	Catchall        bool     `json:"catchall"`
}

// DeleteRoutingRuleRequest deletes a routing rule by id.
type DeleteRoutingRuleRequest struct {
	RoutingRuleID int64 `json:"routingRuleId"`
}

// SplitUserName splits a full username (user@domain.com) into local part and
// domain. It returns an error if the username is not of that form.
func SplitUserName(full string) (local, domain string, err error) {
	for i := 0; i < len(full); i++ {
		if full[i] == '@' {
			return full[:i], full[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("username %q is not of the form user@domain", full)
}
