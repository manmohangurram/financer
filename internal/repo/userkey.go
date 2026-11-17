package repo

// UserKeyRow is a per-user API key as returned to the UI (never the secret).
type UserKeyRow struct {
	ID         string
	Name       string
	KeyPrefix  string
	CreatedAt  string
	ExpiresAt  *string
	LastUsedAt *string
	Scope      string
}

// UserKeyRepo is the backend-agnostic API-key repository.
type UserKeyRepo interface {
	Create(userID, name, keyHash, keyPrefix string, expiresAt *string, scope string) (string, error)
	List(userID string) ([]*UserKeyRow, error)
	Count(userID string) (int64, error)
	GetByID(userID, id string) (*UserKeyRow, error)
	Delete(userID, id string) (bool, error)
	ByKeyHash(keyHash string) (userID, id, scope string, ok bool, err error)
	TouchLastUsed(id string) error
}
