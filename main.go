package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"github.com/matt-horst/blog-agg/internal/config"
	"github.com/matt-horst/blog-agg/internal/database"

	_ "github.com/lib/pq"
)


type state struct {
	cfg *config.Config
	db *database.Queries
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

func (c *commands) register(name string, f func(*state, command) error) {
	c.handlers[name] = f
}

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatalf("Failed to read config file: %v\n", err)
	}

	db, err := sql.Open("postgres", cfg.DbURL)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	s := &state{
		cfg: &cfg,
		db: database.New(db),
	}

	cmds := commands {handlers: make(map[string]func(*state, command) error)}
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	cmds.register("agg", handlerAgg)
	cmds.register("browse", middlewareLoggedIn(handlerBrowse))

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

