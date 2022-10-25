package notifications

type Notifications interface {
	Connect()
	Notify()
	Connected()
}
