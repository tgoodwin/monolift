package unknownkeyfixture

import "context"

//monolift:lift name=sender mode=remote mystery=value
type Sender interface {
	Send(context.Context, string) error
}
