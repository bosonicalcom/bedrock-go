package kafkax

//go:generate go tool mockgen -source=controller.go -destination=kafkatest/controller_mock.go -package=kafkatest

type Controller interface {
	RegisterConsumers(registrar ConsumerRegistrar)
}
