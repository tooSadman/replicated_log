package server

const (
	healthy = iota // c0 == 0
	suspected
	unhealthy
)

type Agent struct {
	statuses map[string]int
}

func NewAgent(replicas []string) *Agent {
	statuses := make(map[string]int)
	for _, replica := range replicas {
		statuses[replica] = suspected
	}
	agent := &Agent{statuses: statuses}
	return agent
}
