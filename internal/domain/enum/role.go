package enum

type Role string

const (
	RoleUser       Role = "user"
	RoleAdmin      Role = "admin"
	RoleSuperAdmin Role = "superadmin"
	RoleCashier    Role = "cashier"
	RoleStaff      Role = "staff"
)
