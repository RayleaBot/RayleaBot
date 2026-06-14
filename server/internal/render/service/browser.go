package service

type closeableRunner interface {
	Close() error
}
