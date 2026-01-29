package register

type RegisterInterface interface {
	Register() error
	Unregister() error
	UpdateRegistration() error
	GetRegistrationStatus() error
}
