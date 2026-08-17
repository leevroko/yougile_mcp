package valueobject

// Role — роль пользователя в проекте (по API).
type Role string

// Роли пользователей.
const (
	RoleWorker   Role = "worker"
	RoleAdmin    Role = "admin"
	RoleObserver Role = "observer"
	RoleCustomer Role = "customer"
)
