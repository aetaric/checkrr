package connections

type Connection interface {
	MatchPath()
	RemoveFile()
	Connect()
	FromConfig()
}
