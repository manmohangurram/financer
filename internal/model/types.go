// Package model holds wire types shared by handlers and the frontend (camelCase).
package model

type AccountType string

const (
	AccountTypeCurrent    AccountType = "CURRENT"
	AccountTypeSavings    AccountType = "SAVINGS"
	AccountTypeLoan       AccountType = "LOAN"
	AccountTypeCreditCard AccountType = "CREDIT_CARD"
)

type AccountRow struct {
	ID            string      `json:"id"`
	BankName      string      `json:"bankName"`
	Nickname      string      `json:"nickname"`
	Balance       float64     `json:"balance"`
	AccountType   AccountType `json:"type"`
	EndingNumbers *string     `json:"endingNumbers,omitempty"`
	CreatedAt     string      `json:"createdAt"`
}

type AccountResponse struct {
	ID            string      `json:"id"`
	BankName      string      `json:"bankName"`
	Nickname      string      `json:"nickname"`
	Balance       float64     `json:"balance"`
	AccountType   AccountType `json:"type"`
	EndingNumbers *string     `json:"endingNumbers,omitempty"`
	CreatedAt     string      `json:"createdAt"`
}

type ReqAccount struct {
	ID            string      `json:"id"`
	BankName      string      `json:"bankName"`
	Nickname      string      `json:"nickname"`
	EndingNumbers string      `json:"endingNumbers"`
	AccountType   AccountType `json:"type"`
}

type ListAccountsResult struct {
	Accounts      []AccountResponse `json:"accounts"`
	NextPageToken string            `json:"nextPageToken"`
}

type CategoryRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

type CategoryResponse struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	MonthlyBudget float64 `json:"monthlyBudget"`
	CreatedAt     string  `json:"createdAt"`
}

// ReqCategoryBudget sets one category's recurring monthly budget. 0 removes it.
type ReqCategoryBudget struct {
	MonthlyBudget float64 `json:"monthlyBudget"`
}

type ReqCategory struct {
	IDs        []string `json:"ids"`
	Categories []ReqCat `json:"categories"`
}

type ReqCat struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	MonthlyBudget *float64 `json:"monthlyBudget,omitempty"`
}

type ListCategoriesQuery struct {
	PageSize  int    `json:"pageSize"`
	PageToken string `json:"pageToken"`
}

type ListCategoriesResult struct {
	Categories    []CategoryResponse `json:"categories"`
	NextPageToken string             `json:"nextPageToken"`
}

type AuthResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	UserID       string `json:"userId"`
	Email        string `json:"email"`
	Name         string `json:"name"`
}

type ProfileResponse struct {
	UserID    string `json:"userId"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarUrl"`
}

type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type ReqProfile struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarUrl"`
}

type ReqPassword struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type BulkResult struct {
	Success   bool     `json:"success"`
	Message   string   `json:"message"`
	FailedIDs []string `json:"failedIds"`
	Skipped   int      `json:"skipped"`
}
