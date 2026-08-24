package model

import (
	"context"
	"log/slog"
	"sort"
	"sync"
)

// Service is the catalogue aggregate.
type Service struct {
	catalog Catalog
	log     *slog.Logger
}

// Deps is what the service is built from.
type Deps struct {
	Catalog Catalog
	Log     *slog.Logger
}

// NewService wires the service over a catalogue.
func NewService(d Deps) *Service {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &Service{catalog: d.Catalog, log: log}
}

// List asks the connected providers what they serve.
//
// Naming a provider that is not connected is an error, because it is a mistake
// with an obvious fix and answering an empty list would look like the provider
// serving nothing. Asking every provider is not: there, one failure is one
// entry carrying its reason, and the rest still answer.
func (s *Service) List(ctx context.Context, in ListInput) (ListOutput, error) {
	if s.catalog == nil {
		return ListOutput{}, errNoCatalog()
	}

	connected, err := s.catalog.Connected(ctx)
	if err != nil {
		return ListOutput{}, err
	}

	ask := connected
	if in.Provider != "" {
		if !contains(connected, in.Provider) {
			return ListOutput{}, errNotConnected(in.Provider, connected)
		}
		ask = []string{in.Provider}
	}

	// Concurrently, because these are calls to four different companies'
	// servers and doing them in sequence makes the settings screen wait for
	// the sum of them.
	out := make([]Provider, len(ask))
	var wg sync.WaitGroup
	for i, id := range ask {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			out[i] = s.ask(ctx, id)
		}(i, id)
	}
	wg.Wait()

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })

	total := 0
	for _, p := range out {
		total += len(p.Models)
	}
	return ListOutput{Providers: out, Total: total}, nil
}

// ask reads one provider's catalogue, turning a failure into a reported one.
func (s *Service) ask(ctx context.Context, id string) Provider {
	models, err := s.catalog.Models(ctx, id)
	if err != nil {
		// Logged as well as returned: the message reaches whoever asked, and
		// the log is where somebody looks when a provider has been quietly
		// failing for a week.
		s.log.Warn("could not read a provider's model catalogue", "provider", id, "err", err)
		return Provider{ID: id, Models: []Model{}, Error: err.Error()}
	}
	if models == nil {
		models = []Model{}
	}
	return Provider{ID: id, Models: models}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
