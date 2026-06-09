package kafkax

//go:generate go tool mockgen -source=controller.go -destination=kafkaxtest/controller_mock.go -package=kafkaxtest

type Controller interface {
	RegisterConsumers(registrar ConsumerRegistrar)
}
