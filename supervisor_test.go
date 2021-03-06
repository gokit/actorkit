package actorkit_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gokit/actorkit/internal"

	"github.com/gokit/actorkit"
)

func TestExponentialBackoffRestartSupervisor(t *testing.T) {
	supervisor := actorkit.ExponentialBackOffRestartStrategy(10, 1*time.Second, nil)

	system, err := actorkit.Ancestor("kit", "localhost", actorkit.Prop{
		ContextLogs: actorkit.NewContextLogFn(func(actor actorkit.Actor) actorkit.Logs {
			return &internal.TLog{}
		}),
	})
	require.NoError(t, err)
	require.NotNil(t, system)

	child1, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
	require.NoError(t, err)
	require.NotNil(t, child1)

	child2, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
	require.NoError(t, err)
	require.NotNil(t, child2)

	require.True(t, isRunning(child1), "currently in %+q", child1.State())
	require.True(t, isRunning(child2), "currently in %+q", child2.State())

	var w sync.WaitGroup
	w.Add(2)
	sub := child1.Watch(func(i interface{}) {
		switch sm := i.(type) {
		case actorkit.ActorSignal:

			switch sm.Signal {
			case actorkit.RESTARTING:
				w.Done()
			case actorkit.RESTARTED:
				w.Done()
			}
		}
	})

	supervisor.Handle(errors.New("bad day"), child1, child1.Actor(), system.Actor())

	require.True(t, isRunning(child1), "currently in %+q", child1.State())
	require.True(t, isRunning(child2), "currently in %+q", child2.State())

	w.Wait()
	sub.Stop()

	child1.Actor().Destroy()
	child2.Actor().Destroy()
}

func TestRestartSupervisor(t *testing.T) {
	supervisor := &actorkit.RestartingSupervisor{}
	system, err := actorkit.Ancestor("kit", "localhost", actorkit.Prop{})
	require.NoError(t, err)
	require.NotNil(t, system)

	child1, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
	require.NoError(t, err)
	require.NotNil(t, child1)

	child2, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
	require.NoError(t, err)
	require.NotNil(t, child2)

	require.True(t, isRunning(child1))
	require.True(t, isRunning(child2))

	var w sync.WaitGroup
	w.Add(2)
	sub := child1.Watch(func(i interface{}) {
		switch sm := i.(type) {
		case actorkit.ActorSignal:
			switch sm.Signal {
			case actorkit.RESTARTING:
				w.Done()
			case actorkit.RESTARTED:
				w.Done()
			}
		}
	})

	supervisor.Handle(errors.New("bad day"), child1, child1.Actor(), system.Actor())

	require.True(t, isRunning(child1))
	require.True(t, isRunning(child2))

	w.Wait()
	sub.Stop()

	child1.Actor().Destroy()
	child2.Actor().Destroy()
}

func TestOneForOneSupervisor(t *testing.T) {
	var supervisingAction func(interface{}) actorkit.Directive

	supervisor := &actorkit.OneForOneSupervisor{
		Max: 30,
		PanicAction: func(i interface{}, addr actorkit.Addr, actor actorkit.Actor) {
			require.NotNil(t, i)
			require.IsType(t, actorkit.PanicEvent{}, i)
		},
		Decider: func(tm interface{}) actorkit.Directive {
			return supervisingAction(tm)
		},
	}

	system, err := actorkit.Ancestor("kit", "localhost", actorkit.Prop{})
	require.NoError(t, err)
	require.NotNil(t, system)

	t.Logf("When supervisor is told destroy")
	{
		child1, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child1)

		child2, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child2)

		require.True(t, isRunning(child1))
		require.True(t, isRunning(child2))

		var w sync.WaitGroup
		w.Add(2)
		sub := child1.Watch(func(i interface{}) {
			switch sm := i.(type) {
			case actorkit.ActorSignal:
				switch sm.Signal {
				case actorkit.DESTRUCTING:
					w.Done()
				case actorkit.DESTROYED:
					w.Done()
				}
			}
		})

		supervisingAction = func(i interface{}) actorkit.Directive {
			return actorkit.DestroyDirective
		}

		supervisor.Handle(errors.New("bad day"), child1, child1.Actor(), system.Actor())

		require.False(t, isRunning(child1))
		require.True(t, isRunning(child2))

		w.Wait()
		sub.Stop()
	}

	t.Logf("When supervisor is told kill")
	{
		child1, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child1)

		child2, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child2)

		require.True(t, isRunning(child1))
		require.True(t, isRunning(child2))

		var w sync.WaitGroup
		w.Add(2)
		sub := child1.Watch(func(i interface{}) {
			switch sm := i.(type) {
			case actorkit.ActorSignal:
				switch sm.Signal {
				case actorkit.KILLING:
					w.Done()
				case actorkit.KILLED:
					w.Done()
				}
			}
		})

		supervisingAction = func(i interface{}) actorkit.Directive {
			return actorkit.KillDirective
		}

		supervisor.Handle(errors.New("bad day"), child1, child1.Actor(), system.Actor())

		require.False(t, isRunning(child1))
		require.True(t, isRunning(child2))

		w.Wait()
		sub.Stop()
		child1.Actor().Destroy()
		child2.Actor().Destroy()
	}

	t.Logf("When supervisor is told stop")
	{
		child1, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child1)

		child2, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child2)

		require.True(t, isRunning(child1))
		require.True(t, isRunning(child2))

		var w sync.WaitGroup
		w.Add(2)
		sub := child1.Watch(func(i interface{}) {
			switch sm := i.(type) {
			case actorkit.ActorSignal:
				switch sm.Signal {
				case actorkit.STOPPING:
					w.Done()
				case actorkit.STOPPED:
					w.Done()
				}
			}
		})

		supervisingAction = func(i interface{}) actorkit.Directive {
			return actorkit.StopDirective
		}

		supervisor.Handle(errors.New("bad day"), child1, child1.Actor(), system.Actor())

		require.False(t, isRunning(child1))
		require.True(t, isRunning(child2))

		w.Wait()
		sub.Stop()
		child1.Actor().Destroy()
		child2.Actor().Destroy()
	}

	t.Logf("When supervisor is told restart")
	{
		child1, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child1)

		child2, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child2)

		require.True(t, isRunning(child1))
		require.True(t, isRunning(child2))

		var w sync.WaitGroup
		w.Add(2)
		sub := child1.Watch(func(i interface{}) {
			switch sm := i.(type) {
			case actorkit.ActorSignal:
				switch sm.Signal {
				case actorkit.RESTARTING:
					w.Done()
				case actorkit.RESTARTED:
					w.Done()
				}
			}
		})

		supervisingAction = func(i interface{}) actorkit.Directive {
			return actorkit.RestartDirective
		}

		supervisor.Handle(errors.New("bad day"), child1, child1.Actor(), system.Actor())

		require.True(t, isRunning(child1))
		require.True(t, isRunning(child2))

		w.Wait()
		sub.Stop()

		child1.Actor().Destroy()
		child2.Actor().Destroy()
	}
}

func TestAllForOneSupervisor(t *testing.T) {
	var supervisingAction func(interface{}) actorkit.Directive

	supervisor := &actorkit.AllForOneSupervisor{
		Max: 30,
		PanicAction: func(i interface{}, addr actorkit.Addr, actor actorkit.Actor) {
			require.NotNil(t, i)
			require.IsType(t, actorkit.PanicEvent{}, i)
		},
		Decider: func(tm interface{}) actorkit.Directive {
			return supervisingAction(tm)
		},
	}

	system, err := actorkit.Ancestor("kit", "localhost", actorkit.Prop{})
	require.NoError(t, err)
	require.NotNil(t, system)

	t.Logf("When supervisor is told destroy")
	{
		child1, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child1)

		child2, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child2)

		require.True(t, isRunning(child1))
		require.True(t, isRunning(child2))

		var w sync.WaitGroup
		w.Add(2)
		sub := child1.Watch(func(i interface{}) {
			switch sm := i.(type) {
			case actorkit.ActorSignal:
				switch sm.Signal {
				case actorkit.DESTRUCTING:
					w.Done()
				case actorkit.DESTROYED:
					w.Done()
				}
			}
		})

		supervisingAction = func(i interface{}) actorkit.Directive {
			return actorkit.DestroyDirective
		}

		supervisor.Handle(errors.New("bad day"), child1, child1.Actor(), system.Actor())

		require.False(t, isRunning(child1))
		require.False(t, isRunning(child2))

		w.Wait()
		sub.Stop()
	}

	t.Logf("When supervisor is told kill")
	{
		child1, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child1)

		child2, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child2)

		require.True(t, isRunning(child1))
		require.True(t, isRunning(child2))

		var w sync.WaitGroup
		w.Add(2)
		sub := child1.Watch(func(i interface{}) {
			switch sm := i.(type) {
			case actorkit.ActorSignal:
				switch sm.Signal {
				case actorkit.KILLING:
					w.Done()
				case actorkit.KILLED:
					w.Done()
				}
			}
		})

		supervisingAction = func(i interface{}) actorkit.Directive {
			return actorkit.KillDirective
		}

		supervisor.Handle(errors.New("bad day"), child1, child1.Actor(), system.Actor())

		require.False(t, isRunning(child1))
		require.False(t, isRunning(child2))

		w.Wait()
		sub.Stop()
		child1.Actor().Destroy()
		child2.Actor().Destroy()
	}

	t.Logf("When supervisor is told stop")
	{
		child1, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child1)

		child2, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child2)

		require.True(t, isRunning(child1))
		require.True(t, isRunning(child2))

		var w sync.WaitGroup
		w.Add(2)
		sub := child1.Watch(func(i interface{}) {
			switch sm := i.(type) {
			case actorkit.ActorSignal:
				switch sm.Signal {
				case actorkit.STOPPING:
					w.Done()
				case actorkit.STOPPED:
					w.Done()
				}
			}
		})

		supervisingAction = func(i interface{}) actorkit.Directive {
			return actorkit.StopDirective
		}

		supervisor.Handle(errors.New("bad day"), child1, child1.Actor(), system.Actor())

		require.False(t, isRunning(child1))
		require.False(t, isRunning(child2))

		w.Wait()
		sub.Stop()
		child1.Actor().Destroy()
		child2.Actor().Destroy()
	}

	t.Logf("When supervisor is told restart")
	{
		child1, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child1)

		child2, err := system.Spawn("basic", actorkit.Prop{Behaviour: &basic{}})
		require.NoError(t, err)
		require.NotNil(t, child2)

		require.True(t, isRunning(child1))
		require.True(t, isRunning(child2))

		var w sync.WaitGroup
		w.Add(2)
		sub := child1.Watch(func(i interface{}) {
			switch sm := i.(type) {
			case actorkit.ActorSignal:
				switch sm.Signal {
				case actorkit.RESTARTING:
					w.Done()
				case actorkit.RESTARTED:
					w.Done()
				}
			}
		})

		supervisingAction = func(i interface{}) actorkit.Directive {
			return actorkit.RestartDirective
		}

		supervisor.Handle(errors.New("bad day"), child1, child1.Actor(), system.Actor())

		require.True(t, isRunning(child1))
		require.True(t, isRunning(child2))

		w.Wait()
		sub.Stop()

		child1.Actor().Destroy()
		child2.Actor().Destroy()
	}
}
