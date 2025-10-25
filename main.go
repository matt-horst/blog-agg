package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/matt-host/blog-agg/internal/database"

	_ "github.com/lib/pq"
	"github.com/matt-host/blog-agg/internal/config"
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
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", handlerAddFeed)

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

	name :=  cmd.args[0]

	user, err := s.db.GetUser(context.Background(), name)
	if err != nil {
		return fmt.Errorf("Unable to find user: %v\n", err)
	}

	err = s.cfg.SetUser(user.Name)
	if err != nil {
		return fmt.Errorf("Failed to set the new user: %v\n", err)
	}

	fmt.Printf("New user set to `%s`\n", user.Name)

	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("Username is a required argument\n")
	}

	name := cmd.args[0]

	params := database.CreateUserParams{
		ID: uuid.New(),
		Name: name,
	}
	_, err := s.db.CreateUser(context.Background(), params)
	if err != nil {
		return fmt.Errorf("Failed to create new user: %v\n", err)
	}

	err = s.cfg.SetUser(name)
	if err != nil {
		return fmt.Errorf("Failed to user: %v\n", err)
	}

	fmt.Printf("New user successfully created: %s\n", name)

	return nil
}

func handlerReset(s *state, cmd command) error {
	err := s.db.ResetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to reset users table: %v\n", err)
	}

	return nil
}

func handlerUsers(s *state, cmd command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get users from database: %v\n", err)
	}

	// Print all users to console
	for _, user := range users {
		current := ""
		if user.Name == s.cfg.CurrentUserName {
			current = " (current)"
		} 

		fmt.Printf("* %s%s\n", user.Name, current)
	}

	return nil
}

type RSSFeed struct {
	Channel struct {
		Title 		string 		`xml:"title"`
		Link 		string 		`xml:"link"`
		Description string 		`xml:"description"`
		Item 		[]RSSItem 	`xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title 		string `xml:"title"`
	Link 		string `xml:"link"`
	Description string `xml:"description"`
	PubDate 	string `xml:"pubDate"`
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create GET request for URL `%s`: %v\n", feedURL, err)
	}

	req.Header.Set("User-Agent", "gator")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to get response from `%s`: %v\n", feedURL, err)
	}

	rssFeed := &RSSFeed{}
	decoder := xml.NewDecoder(resp.Body)
	err = decoder.Decode(rssFeed)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode rss feed xml: %v\n", err)
	}

	// Unescape HTML entities
	rssFeed.Channel.Title = html.UnescapeString(rssFeed.Channel.Title)
	rssFeed.Channel.Description = html.UnescapeString(rssFeed.Channel.Description)
	for i, item := range rssFeed.Channel.Item {
		rssFeed.Channel.Item[i].Title = html.UnescapeString(item.Title)
		rssFeed.Channel.Item[i].Description = html.UnescapeString(item.Description)
	}

	return rssFeed, nil
}

func handlerAgg(s *state, cmd command) error {
	url := "https://www.wagslane.dev/index.xml"
	rssFeed, err := fetchFeed(context.Background(), url)
	if err != nil {
		return err
	}

	fmt.Println(rssFeed)

	return nil
}

func handlerAddFeed(s *state, cmd command) error {
	if len(cmd.args) != 2 {
		return fmt.Errorf("addfeed requires two arguments: name url")
	}

	name := cmd.args[0]
	url := cmd.args[1]

	user, err := s.db.GetUser(context.Background(), s.cfg.CurrentUserName)
	if err != nil {
		return err
	}

	params := database.CreateFeedParams{
		ID: uuid.New(),
		Name: name,
		Url: url,
		UserID: user.ID,
	}
	feed, err := s.db.CreateFeed(
		context.Background(),
		params,
	)
	if err != nil {
		return fmt.Errorf("Failed to create new feed: %v\n", err)
	}

	fmt.Println(feed)

	return nil
}
