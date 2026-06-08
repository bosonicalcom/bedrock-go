package kafkax

type Controller interface {
	RegisterConsumers(registrar ConsumerRegistrar)
}
