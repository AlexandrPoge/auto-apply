package model

type Counter struct {
	Value int
}

func (c *Counter) Increment() {
	c.Value++
}
