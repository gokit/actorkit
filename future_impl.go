package actorkit

import (
	"sync"
	"time"

	"github.com/gokit/es"

	"github.com/gokit/errors"
	"github.com/gokit/xid"
)

// errors ...
var (
	ErrFutureTimeout          = errors.New("Future timed out")
	ErrFutureResolved         = errors.New("Future is resolved")
	ErrFutureEscalatedFailure = errors.New("Future failed due to escalated error")
)

var (
	_ Future = &FutureImpl{}
)

// FutureImpl defines an implementation the Future interface, it provides a type of address
// which defers the resolution of a underline result into the future and can be passed around like
// any address, where the result can be later retrieved using pipeTo feature.
type FutureImpl struct {
	id     xid.ID
	parent Addr
	timer  *time.Timer
	events *es.EventStream

	ac    sync.Mutex
	pipes []func(Envelope)

	w      sync.WaitGroup
	cw     sync.Mutex
	err    error
	result *Envelope
}

// NewFuture returns a new instance of giving future.
func NewFuture(parent Addr) *FutureImpl {
	var ft FutureImpl
	ft.parent = parent
	ft.events = es.New()
	ft.w.Add(1)
	return &ft
}

// TimedFuture returns a new instance of giving future.
func TimedFuture(parent Addr, dur time.Duration) *FutureImpl {
	var ft FutureImpl
	ft.parent = parent
	ft.events = es.New()
	ft.timer = time.NewTimer(dur)
	ft.w.Add(1)

	go ft.timedResolved()
	return &ft
}

// Forward delivers giving envelope into Future actor which if giving
// future is not yet resolved will be the resolution of future.
func (f *FutureImpl) Forward(reply Envelope) error {
	if f.resolved() {
		return errors.Wrap(ErrFutureResolved, "Future %q already resolved", f.Addr())
	}

	f.Resolve(reply)
	f.broadcast()
	return nil
}

// Send delivers giving data to resolve the future.
//
// If data is a type of error then the giving future is
// rejected.
func (f *FutureImpl) Send(data interface{}, addr Addr) error {
	if f.resolved() {
		return errors.Wrap(ErrFutureResolved, "Future %q already resolved", f.Addr())
	}

	f.Resolve(CreateEnvelope(addr, Header{}, data))
	f.broadcast()
	return nil
}

// SendWithHeader delivers giving data to Future as the resolution of
// said Future. The data provided will be used as the resolved
// value of giving future, if it's not already resolved.
//
// If data is a type of error then the giving future is
// rejected.
func (f *FutureImpl) SendWithHeader(data interface{}, h Header, addr Addr) error {
	if f.resolved() {
		return errors.Wrap(ErrFutureResolved, "Future %q already resolved", f.Addr())
	}

	f.Resolve(CreateEnvelope(addr, h, data))
	f.broadcast()
	return nil
}

// Escalate escalates giving value into the parent of giving future, which also fails future
// and resolves it as a failure.
func (f *FutureImpl) Escalate(m interface{}) {
	var data EscalatedError
	data.Err = ErrFutureEscalatedFailure
	data.Value = m

	if merr, ok := m.(error); ok {
		data.Err = merr
	}

	f.Resolve(CreateEnvelope(DeadLetters(), Header{}, data))
	f.broadcast()
}

// AddressOf requests giving service from future's parent AddressOf method.
func (f *FutureImpl) AddressOf(service string, ancestry bool) (Addr, error) {
	return nil, errors.New("not possible")
}

// Spawn requests giving service and Receiver from future's parent Spawn method.
func (f *FutureImpl) Spawn(service string, ops Prop) (Addr, error) {
	return nil, errors.New("not possible")
}

// Actor for a future does not exists, it is in a sense actor less,
// hence nil is returned.
func (f *FutureImpl) Actor() Actor {
	return nil
}

// Future returns a new future instance from giving source.
func (f *FutureImpl) Future() Future {
	return NewFuture(f.parent)
}

// TimedFuture returns a new future instance from giving source.
func (f *FutureImpl) TimedFuture(d time.Duration) Future {
	return TimedFuture(f.parent, d)
}

// Watch adds giving function into event system for future.
func (f *FutureImpl) Watch(fn func(interface{})) Subscription {
	return subscriber{f.events.Subscribe(fn)}
}

// DeathWatch implements DeathWatch interface.
func (f *FutureImpl) DeathWatch(addr Addr) error {
	return errors.WrapOnly(ErrHasNoActor)
}

// Parent returns the address of the parent of giving Future.
func (f *FutureImpl) Parent() Addr {
	return f.parent
}

// Ancestor returns the root parent address of the of giving Future.
func (f *FutureImpl) Ancestor() Addr {
	return f.parent.Ancestor()
}

// State returns state of actor.
func (f *FutureImpl) State() Signal {
	if !f.resolved() {
		return RUNNING
	}
	if f.fulfilled() {
		return RESOLVED
	}
	return REJECTED
}

// Children returns an empty slice as futures can not have children actors.
func (f *FutureImpl) Children() []Addr {
	return nil
}

// GetAddr implements the Descendant interface but futures are not allowed to have children.
func (f *FutureImpl) GetAddr(addr string) (Addr, error) {
	return nil, errors.New("has no children")
}

// GetChild implements the Descendant interface but futures are not allowed to have children.
func (f *FutureImpl) GetChild(id string, subID ...string) (Addr, error) {
	return nil, errors.New("has no children")
}

// Service returns the "Future" as the service name of FutureImpl.
func (f *FutureImpl) Service() string {
	return "Future"
}

// ID returns the unique id of giving Future.
func (f *FutureImpl) ID() string {
	return f.id.String()
}

// Namespace returns future's parent namespace value.
func (f *FutureImpl) Namespace() string {
	return f.parent.Namespace()
}

// Protocol returns future's parent's protocol value.
func (f *FutureImpl) Protocol() string {
	return f.parent.Protocol()
}

// Addr returns s consistent address format representing a future addr.
func (f *FutureImpl) Addr() string {
	return formatAddrService2(FormatAddrChild(f.parent.Addr(), f.ID()), f.Service())
}

// ProtocolAddr implements the ProtocolAddr interface. It
// always returns future@parent_namespace/service.
func (f *FutureImpl) ProtocolAddr() string {
	return f.parent.ProtocolAddr()
}

// Wait blocks till the giving future is resolved and returns error if
// occurred.
func (f *FutureImpl) Wait() error {
	f.w.Wait()
	return f.Err()
}

// PipeAction allows the addition of functions to be called with result of
// future.
func (f *FutureImpl) PipeAction(actions ...func(envelope Envelope)) {
	if f.resolved() {
		for _, action := range actions {
			action(*f.result)
		}
		return
	}

	f.ac.Lock()
	f.pipes = append(f.pipes, actions...)
	f.ac.Unlock()
}

// Pipe adds giving set of address into giving Future.
func (f *FutureImpl) Pipe(addrs ...Addr) {
	if f.resolved() {
		for _, addr := range addrs {
			addr.Forward(*f.result)
		}
		return
	}

	for _, addr := range addrs {
		func(a Addr) {
			f.PipeAction(func(msg Envelope) {
				a.Forward(msg)
			})
		}(addr)
	}
}

// Err returns the error for the failure of
// giving error.
func (f *FutureImpl) Err() error {
	var merr error
	f.cw.Lock()
	merr = f.err
	f.cw.Unlock()
	return merr
}

// Result returns the envelope which is used to resolve the future.
func (f *FutureImpl) Result() Envelope {
	f.cw.Lock()
	defer f.cw.Unlock()
	return *f.result
}

// Resolve resolves giving future with envelope.
func (f *FutureImpl) Resolve(env Envelope) {
	if f.resolved() {
		return
	}

	var ok bool
	var err error
	var rejected bool

	f.cw.Lock()
	if err, ok = env.Data.(error); ok {
		rejected = true
		f.err = err
	}

	f.result = &env
	f.cw.Unlock()
	f.w.Done()

	if rejected {
		f.events.Publish(FutureRejected{Err: err, ID: f.id.String()})
		return
	}
	f.events.Publish(FutureResolved{Data: env, ID: f.id.String()})
}

func (f *FutureImpl) resolved() bool {
	f.cw.Lock()
	defer f.cw.Unlock()
	return f.result != nil || f.err != nil
}

func (f *FutureImpl) rejected() bool {
	f.cw.Lock()
	defer f.cw.Unlock()
	return f.result != nil && f.err != nil
}

func (f *FutureImpl) fulfilled() bool {
	f.cw.Lock()
	defer f.cw.Unlock()
	return f.result != nil && f.err == nil
}

func (f *FutureImpl) broadcast() {
	var res *Envelope

	f.ac.Lock()
	res = f.result
	f.ac.Unlock()

	for _, action := range f.pipes {
		action(*res)
	}

	f.pipes = nil
}

func (f *FutureImpl) timedResolved() {
	<-f.timer.C
	if f.resolved() {
		return
	}
	f.Escalate(ErrFutureTimeout)
}
