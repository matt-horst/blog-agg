package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"
	"encoding/xml"
	"html"
	"net/http"
	"strings"
	"database/sql"

	"github.com/matt-horst/blog-agg/internal/database"

	"github.com/google/uuid"
)

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(s *state, cmd command) error {
	return func(s *state, cmd command) error {
		user, err := s.db.GetUser(context.Background(), s.cfg.CurrentUserName)
		if err != nil {
			return fmt.Errorf("Unable to find user `%s`: %v", s.cfg.CurrentUserName, err)
		}

		return handler(s, cmd, user)
	}
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("Username is required")
	}

	name :=  cmd.args[0]

	user, err := s.db.GetUser(context.Background(), name)
	if err != nil {
		return fmt.Errorf("Unable to find user: %v", err)
	}

	err = s.cfg.SetUser(user.Name)
	if err != nil {
		return fmt.Errorf("Failed to set the new user: %v", err)
	}

	fmt.Printf("New user set to `%s`\n", user.Name)

	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("Username is a required argument")
	}

	name := cmd.args[0]

	params := database.CreateUserParams{
		ID: uuid.New(),
		Name: name,
	}
	_, err := s.db.CreateUser(context.Background(), params)
	if err != nil {
		return fmt.Errorf("Failed to create new user: %v", err)
	}

	err = s.cfg.SetUser(name)
	if err != nil {
		return fmt.Errorf("Failed to user: %v", err)
	}

	fmt.Printf("New user successfully created: %s\n", name)

	return nil
}

func handlerReset(s *state, cmd command) error {
	err := s.db.ResetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to reset users table: %v", err)
	}

	return nil
}

func handlerUsers(s *state, cmd command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get users from database: %v", err)
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


func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 2 {
		return fmt.Errorf("addfeed requires two arguments: name url")
	}

	name := cmd.args[0]
	url := cmd.args[1]

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
		return fmt.Errorf("Failed to create new feed: %v", err)
	}

	_, err = s.db.CreateFeedFollow(
		context.Background(),
		database.CreateFeedFollowParams{
			ID: uuid.New(),
			UserID: user.ID,
			FeedID: feed.ID,
		},
	)
	if err != nil {
		return fmt.Errorf("Failed to follow new feed: %v", err)
	}

	fmt.Println(feed)

	return nil
}

func handlerFeeds(s *state, cmd command) error {
	feeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get feeds from database: %v", err)
	}

	for _, feed := range feeds {
		fmt.Printf("* %s %s %s\n", feed.Name, feed.Url, feed.UserName)
	}

	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("follow command requires url argument")
	}

	url := cmd.args[0]

	feed, err := s.db.GetFeed(context.Background(), url)
	if err != nil {
		return fmt.Errorf("Unable to find feed `%s`: %v", url, err)
	}

	feedFollow, err := s.db.CreateFeedFollow(
		context.Background(),
		database.CreateFeedFollowParams{
			ID: uuid.New(),
			UserID: user.ID,
			FeedID: feed.ID,
		},
	)
	if err != nil {
		return fmt.Errorf("Failed to create feed-follow: %v", err)
	}

	fmt.Printf("%s - %s\n", feedFollow.FeedName, feedFollow.UserName)

	return nil
}

func handlerFollowing(s *state, _ command, _ database.User) error {
	following, err := s.db.GetFeedFollowsForUser(context.Background(), s.cfg.CurrentUserName)
	if err != nil {
		return fmt.Errorf("Unable to find user `%s`: %v", s.cfg.CurrentUserName, err)
	}

	for _, f := range following {
		fmt.Printf("* %s\n", f.FeedName)
	}

	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("unfollow requires url as argument")
	}

	url := cmd.args[0]

	feed, err := s.db.GetFeed(context.Background(), url)
	if err != nil {
		return fmt.Errorf("Unable to find feed for `%s`: %v", url, err)
	}

	_, err = s.db.DeleteFeedFollow(
		context.Background(),
		database.DeleteFeedFollowParams{
			UserID: user.ID,
			FeedID: feed.ID,
		},
	)
	if err != nil {
		return fmt.Errorf("Unable to find feed-follow: %v", err)
	}

	return nil
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("agg requires time between requests argument")
	}

	timeBetweenReqs, err := time.ParseDuration(cmd.args[0])
	if err != nil {
		return fmt.Errorf("Failed to parse `%s` as a duration: %v", cmd.args[0], err)
	}

	fmt.Printf("Printing feeds every %v\n", timeBetweenReqs)

	ticker := time.NewTicker(timeBetweenReqs)

	for ; ; <-ticker.C {
		err = scrapeFeeds(s)
		if err != nil {
			log.Printf("Failed to scrape feeds: %v\n", err)
		}
	}
}

func handlerBrowse(s *state, cmd command, user database.User) error {
	limit := 2
	if len(cmd.args) == 1 {
		arg, err := strconv.Atoi(cmd.args[0])
		if err != nil {
			return fmt.Errorf("Failed to convert `%s` to integer", cmd.args[0])
		}
		limit = arg
	}

	posts, err := s.db.GetPostsForUser(
		context.Background(),
		database.GetPostsForUserParams{
			UserID: user.ID,
			Limit: int32(limit),
		},
	)
	if err != nil {
		return fmt.Errorf("Failed to get posts for user")
	}

	fmt.Printf("Found %d posts for you!\n", len(posts))
	for _, p := range posts {
		fmt.Printf("%s\n", p.PublishedAt.Time.Format("Mon Jan 2, 2006"))
		fmt.Printf("*** %s ***\n", p.Title)
		fmt.Printf("	%v\n", p.Description)
		fmt.Printf("Link: %s\n", p.Url)
		fmt.Println()
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
		return nil, fmt.Errorf("Failed to create GET request for URL `%s`: %v", feedURL, err)
	}

	req.Header.Set("User-Agent", "gator")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to get response from `%s`: %v", feedURL, err)
	}

	rssFeed := &RSSFeed{}
	decoder := xml.NewDecoder(resp.Body)
	err = decoder.Decode(rssFeed)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode rss feed xml: %v", err)
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

func scrapeFeeds(s *state) error {
	feed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get next feed: %v", err)
	}

	err = s.db.MarkFeedFetched(context.Background(), feed.ID)
	if err != nil {
		return fmt.Errorf("Failed to mark feed as fetched: %v", err)
	}

	fetchedFeed, err := fetchFeed(context.Background(), feed.Url)
	if err != nil {
		return fmt.Errorf("Failed to fetch feed: %v", err)
	}

	for _, item := range fetchedFeed.Channel.Item {
		title := html.UnescapeString(item.Title)
		description := html.UnescapeString(item.Description)
		link := html.UnescapeString(item.Link)

		publishedAt := sql.NullTime{}
		publishedAtTime, err := parseTime(html.UnescapeString(item.PubDate))
		if err != nil {
			log.Printf("Failed to convert publication date `%s` to time format: %v\n", item.PubDate, err)
		} else {
			publishedAt.Time = publishedAtTime
			publishedAt.Valid = true
		}
		p, err := s.db.CreatePost(
			context.Background(), 
			database.CreatePostParams{
				ID: uuid.New(),
				Title: title,
				Description: description,
				PublishedAt: publishedAt,
				Url: link,
				FeedID: feed.ID,
			},
		)

		if strings.Contains(err.Error(), "duplicate key") {
			log.Printf("Post for `%s` already exists", link)
			continue
		}
		if err != nil {
			log.Printf("Failed to create post: %v\n", err)
			continue
		}

		fmt.Printf("New post created for `%s`: %s (%s)\n", p.Title, p.Url, p.PublishedAt.Time.String())
	}

	return nil
}

func parseTime(str string) (time.Time, error) {
	formats := []string{
		time.Layout,
		time.ANSIC,
		time.UnixDate,
		time.RubyDate,
		time.RFC822,
		time.RFC822Z,
		time.RFC850,
		time.RFC1123,
		time.RFC1123Z,
		time.RFC3339,
		time.RFC3339Nano,
		time.Kitchen,
	}

	for _, f := range formats {
		parsed, err := time.Parse(f, str)
		if err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("No valid time format found for `%s`", str)
}
