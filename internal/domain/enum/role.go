package enum

type Role string

const (
	RoleUser       Role = "user"
	RoleAdmin      Role = "admin"
	RoleSuperAdmin Role = "superadmin"
	RoleCashier    Role = "cashier"
	RoleStaff      Role = "staff"
	// E-commerce admin roles (migration 000048, Bu Santi 20 Jul 2026).
	// ecom_admin: manage produk online + order online, no user mgmt.
	// ecom_superadmin: + manage ecom admin users di scope ecom saja.
	// superadmin: bisa akses semua sistem (POS + ecom).
	RoleEcomAdmin      Role = "ecom_admin"
	RoleEcomSuperAdmin Role = "ecom_superadmin"
)
