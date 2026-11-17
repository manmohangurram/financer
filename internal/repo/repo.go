// Package repo defines backend-agnostic repository interfaces.
package repo

// ---- users ----

type UserRow struct {
	ID           string
	Email        string
	Name         *string
	PasswordHash *string
	TokenVersion int64
	AvatarURL    *string
}

type UserRepo interface {
	Create(email, passwordHash, name string) (string, error)
	ByEmail(email string) (*UserRow, error)
	ByID(id string) (*UserRow, error)
	UpdateProfile(id, name, email, avatarURL string) error
	ChangePassword(id, passwordHash string) (int64, error)
	LogoutAll(id string) error
}

// ---- accounts ----

type AccountRow struct {
	ID            string
	BankName      string
	Nickname      string
	Balance       float64
	AccountType   string
	EndingNumbers *string
	CreatedAt     string
}

type AccountRepo interface {
	Create(userID, bankName, nickname, accountType, endingNumbers string) (*AccountRow, error)
	GetByID(userID, id string) (*AccountRow, error)
	Update(userID, id, bankName, nickname, accountType, endingNumbers string) (*AccountRow, error)
	Delete(userID, id string) (bool, error)
	List(userID string) ([]*AccountRow, error)
	SumBalance(userID string) (float64, error)
	SumTotals(userID string) (credit, debit float64, err error)
}

// ---- categories ----

type CategoryRow struct {
	ID            string
	Name          string
	MonthlyBudget float64
	CreatedAt     string
}

type CategoryCreateInput struct {
	Name string
}

type CategoryUpdateInput struct {
	ID   string
	Name string
}

type ListCategoriesResult struct {
	Categories    []*CategoryRow
	NextPageToken string
}

type CategoryRepo interface {
	Create(userID string, inputs []CategoryCreateInput) ([]*CategoryRow, error)
	GetByID(userID, id string) (*CategoryRow, error)
	Update(userID string, inputs []CategoryUpdateInput) ([]string, error)
	Delete(userID string, ids []string) ([]string, error)
	List(userID string, pageSize int64, pageToken string) (*ListCategoriesResult, error)
	SetBudget(userID, id string, budget float64) (bool, error)
}
