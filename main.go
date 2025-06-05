package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/necodeus/gator/internal/config"
	"github.com/necodeus/gator/internal/database"
)

type state struct {
	db     *database.Queries
	Config *config.Config
}

type command struct {
	Name string
	Args []string
}

type commands struct {
	Login    func(s *state, cmd command) error
	Register func(s *state, cmd command) error
}

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "gator")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad response status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var feed RSSFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, fmt.Errorf("unmarshalling XML: %w", err)
	}

	// Decode HTML entities in feed metadata
	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	for i := range feed.Channel.Item {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
	}

	return &feed, nil
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.Args) == 0 {
		return fmt.Errorf("login command requires a username")
	}

	// check if user is in the database
	ctx := context.Background()
	users, err := s.db.GetUsersByName(ctx, cmd.Args[0])
	if err != nil {
		if err != sql.ErrNoRows {
			return fmt.Errorf("failed to get user: %v", err)
		}
	}
	if len(users) == 0 {
		return fmt.Errorf("user %s does not exist", cmd.Args[0])
	}

	fmt.Println("Logging in user...", users[0].Name)
	s.Config.CurrentUserName = users[0].Name

	// nadpisz ustawienie
	if err := config.Write(*s.Config); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.Args) == 0 {
		return fmt.Errorf("register command requires a username")
	}

	userToRegister := cmd.Args[0]

	fmt.Println("Registering user...")

	fmt.Printf("User to register: %s\n", userToRegister)
	users, err := s.db.GetUsersByName(context.Background(), userToRegister)
	if err != nil {
		if err != sql.ErrNoRows {
			return fmt.Errorf("failed to get user: %v", err)
		}
	}

	if len(users) > 0 {
		return fmt.Errorf("user %s already exists", userToRegister)
	}

	ctx := context.Background()

	data := database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      userToRegister,
	}

	user, err := s.db.CreateUser(ctx, data)
	if err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}

	fmt.Println("Logging in user...", user.Name)
	s.Config.CurrentUserName = user.Name

	// nadpisz ustawienie
	if err := config.Write(*s.Config); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	return nil
}

func handlerReset(s *state, cmd command) error {
	fmt.Println("Resetting database...")

	// Delete all users
	ctx := context.Background()
	if err := s.db.DeleteUsers(ctx); err != nil {
		return fmt.Errorf("failed to delete users: %v", err)
	}

	fmt.Println("Database reset successfully.")

	return nil
}

func handlerUsers(s *state, cmd command) error {
	ctx := context.Background()
	users, err := s.db.GetUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get users: %v", err)
	}

	currentUser := s.Config.CurrentUserName

	for _, user := range users {
		if user.Name == currentUser {
			fmt.Printf("* %s (current)\n", user.Name)
		} else {
			fmt.Printf("* %s\n", user.Name)
		}
	}

	return nil
}

func handlerAgg(s *state, cmd command) error {
	feed, err := fetchFeed(context.Background(), "https://www.wagslane.dev/index.xml")
	if err != nil {
		return fmt.Errorf("failed to fetch feed: %v", err)
	}

	for _, item := range feed.Channel.Item {
		fmt.Printf("- %s\n", item.Title)
	}

	return nil
}

func handlerAddFeed(s *state, cmd command) error {
	// 2 args
	if len(cmd.Args) < 2 {
		return fmt.Errorf("addfeed command requires a feed URL and a user ID")
	}

	ctx := context.Background()

	// rss, err := fetchFeed(ctx, cmd.Args[1])
	// if err != nil {
	// 	return fmt.Errorf("failed to fetch feed: %v", err)
	// }

	feeds, err := s.db.GetFeedsByName(ctx, cmd.Args[0])
	if err != nil {
		if err != sql.ErrNoRows {
			return fmt.Errorf("failed to get feed: %v", err)
		}
	}

	if len(feeds) > 0 {
		return fmt.Errorf("feed %s already exists", cmd.Args[0])
	}

	currentUser := s.Config.CurrentUserName
	users, err := s.db.GetUsersByName(ctx, currentUser)
	if err != nil {
		if err != sql.ErrNoRows {
			return fmt.Errorf("failed to get user: %v", err)
		}
	}

	_, err = s.db.CreateFeed(ctx, database.CreateFeedParams{
		ID:     uuid.New(),
		UserID: users[0].ID,
		Name:   cmd.Args[0],
		Url:    cmd.Args[1],
	})
	if err != nil {
		return fmt.Errorf("failed to create feed: %v", err)
	}

	return nil
}

func (c *commands) run(s *state, cmd command) error {
	switch cmd.Name {
	case "login":
		return handlerLogin(s, cmd)
	case "register":
		return handlerRegister(s, cmd)
	case "reset":
		return handlerReset(s, cmd)
	case "users":
		return handlerUsers(s, cmd)
	case "agg":
		return handlerAgg(s, cmd)
	case "addfeed":
		return handlerAddFeed(s, cmd)
	default:
		return fmt.Errorf("unknown command: %s", cmd.Name)
	}
}

func main() {
	// Load configuration

	cfg := config.Config{}
	config, err := cfg.Read()
	if err != nil {
		panic(err)
	}
	cfg = config

	fmt.Printf("Config loaded: %+v\n", config)

	// Initialize database connection

	db, err := sql.Open("postgres", cfg.DbUrl)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}

	// State initialization

	s := &state{
		Config: &config,
		db:     database.New(db),
	}

	// Process command line arguments

	args := os.Args[1:]
	if len(args) < 1 {
		fmt.Println("Usage: gator <command> [args]")
		os.Exit(1)
	}

	cmd := command{
		Name: args[0],
		Args: args[1:],
	}

	c := &commands{}
	err = c.run(s, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
