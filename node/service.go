package node

import (
	"errors"
	"reflect"

	"github.com/kowala-tech/equilibrium/p2p"
	"github.com/kowala-tech/kcoin/client/event"
)

var (
	errServiceUnknown = errors.New("unknown service")
)

// ServiceContext is a collection of service independent options inherited from
// the protocol stack, that is passed to all constructors to be optionally used;
// as well as utility methods to operate on the service environment.
type ServiceContext struct {
	cfg            *Config
	services       map[reflect.Type]Service
	GlobalEventMux *event.TypeMux
}

// Service retrieves a currently running service registered of a specific type.
func (ctx *ServiceContext) Service(service interface{}) error {
	element := reflect.ValueOf(service).Elem()
	if running, ok := ctx.services[element.Type()]; ok {
		element.Set(reflect.ValueOf(running))
		return nil
	}
	return errServiceUnknown
}

// ServiceConstructor is the function signature of the constructors needed to be
// registered for service instantiation.
type ServiceConstructor func(ctx *ServiceContext) (Service, error)

// Service is an individual protocol that can be registered into a node.
type Service interface {
	Start(server *p2p.Host) error
	Stop() error
}
