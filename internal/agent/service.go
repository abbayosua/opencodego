package agent

import (
	"fmt"
	"sort"
)

type Service struct {
	agents       map[string]*Info
	defaultAgent string
}

func NewService() *Service {
	s := &Service{
		agents: BuiltInAgents(),
	}

	// Prefer "build" as default, fallback to first non-subagent
	if a, ok := s.agents["build"]; ok && a.Mode != ModeSubagent && !a.Hidden {
		s.defaultAgent = "build"
	} else {
		for _, a := range s.agents {
			if a.Mode != ModeSubagent && !a.Hidden {
				s.defaultAgent = a.Name
				break
			}
		}
	}

	return s
}

func (s *Service) Get(name string) (*Info, error) {
	a, ok := s.agents[name]
	if !ok {
		return nil, fmt.Errorf("agent %q not found", name)
	}
	return a, nil
}

func (s *Service) List() []*Info {
	var list []*Info
	for _, a := range s.agents {
		list = append(list, a)
	}

	sort.Slice(list, func(i, j int) bool {
		// Default agent first
		if list[i].Name == s.defaultAgent {
			return true
		}
		if list[j].Name == s.defaultAgent {
			return false
		}
		return list[i].Name < list[j].Name
	})

	return list
}

func (s *Service) DefaultAgent() string {
	return s.defaultAgent
}

func (s *Service) SetDefaultAgent(name string) error {
	if _, ok := s.agents[name]; !ok {
		return fmt.Errorf("agent %q not found", name)
	}
	s.defaultAgent = name
	return nil
}

func (s *Service) Register(info *Info) error {
	if info.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if info.Mode == "" {
		info.Mode = ModeAll
	}
	if info.Permission == nil {
		info.Permission = DefaultPermissions
	}
	s.agents[info.Name] = info
	return nil
}

func (s *Service) ListByMode(mode Mode) []*Info {
	var list []*Info
	for _, a := range s.agents {
		if a.Mode == mode || a.Mode == ModeAll {
			list = append(list, a)
		}
	}
	return list
}
