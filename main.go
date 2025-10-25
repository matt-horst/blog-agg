package main

import (
	"fmt"
	"log"
	"os"

	"github.com/matt-host/blog-agg/internal/config"
	_ "github.com/lib/pq"
)


type state struct {
	cfg *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	handlers map[string]func (*state, command) error
}

func (c *commands) run(s *state, cmd command) error {
	handler, ok := c.handlers[cmd.name]
	if !ok {
		return fmt.Errorf("No such command `%s`\n", cmd.name)
	}

	err := handler(s, cmd)
	if err != nil {
		return fmt.Errorf("Failed to run handler for `%s`: %v", cmd.name, err)
	}

	return nil
}

func (c *commands) regiseter(name string, f func(*state, command) error) {
	c.handlers[name] = f
}

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatalf("Failed to read config file: %v\n", err)
	}

	s := &state{cfg: &cfg}

	cmds := commands {handlers: make(map[string]func(*state, command) error)}
	cmds.regiseter("login", handlerLogin)

	if len(os.Args) < 2 {
		log.Fatalf("Requires at least 2 args\n")
	}

	name := os.Args[1]
	args := os.Args[2:]

	cmd := command {name: name, args: args}

	err = cmds.run(s, cmd)
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("Username is required\n")
	}

	user :=  cmd.args[0]

	err := s.cfg.SetUser(user)
	if err != nil {
		return fmt.Errorf("Failed to set the new user: %v\n", err)
	}

	fmt.Printf("New user set to `%s`\n", user)

	return nil
}
