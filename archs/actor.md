Actors
----------

## Hello World

```go
import (
	"fmt"

	"github.com/gokit/actorkit"
)

type HelloMessage struct {
	Name string
}

type HelloOp struct {
	
}

func (h *HelloOp) Action(me actorkit.Addr, e actorkit.Envelope) {
	switch mo := e.Data.(type) {
	case HelloMessage:
		fmt.Printf("Hello World %q\n", mo.Name)
	}
}

func main() {
	addr,  err := actorkit.System(&HelloOp{}, "kit", "localhos:0", nil)
	if err != nil {
		panic(err)
	}

	addr.Send(HelloMessage{Name: "Wally"}, actorkit.Header{}, actorkit.DeadLetters())
	actorkit.Poison(addr)
}
```