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

	"github.com/Dragonicorn/gator/internal/config"
	"github.com/Dragonicorn/gator/internal/database"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type state struct {
	db     *database.Queries
	config *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	handler map[string]func(*state, command) error
}

// Register new handler function for command name
func (c *commands) register(name string, f func(*state, command) error) {
	c.handler[name] = f
	return
}

// Run command with provided state
func (c *commands) run(s *state, cmd command) error {
	err := c.handler[cmd.name](s, cmd)
	return err
}

func handlerreset(s *state, cmd command) error {
	var (
		ct  context.Context = context.Background()
		err error
	)
	if len(cmd.args) > 0 {
		fmt.Println("reset command requires no arguments")
		return fmt.Errorf("reset command requires no arguments\n")
	}
	// Delete all users from database
	err = s.db.DeleteUsers(ct)
	if err != nil {
		fmt.Println("Unable to remove all users from database")
		return fmt.Errorf("reset command database delete query error: %v\n", err)
	}
	fmt.Println("All users have been removed from database")
	return nil
}

func handlerlogin(s *state, cmd command) error {
	var (
		ct  context.Context = context.Background()
		err error
	)
	if len(cmd.args) == 0 {
		fmt.Println("login command requires username")
		return fmt.Errorf("login command requires username\n")
	}
	// Check for existing user in database
	_, err = s.db.GetUser(ct, cmd.args[0])
	if err != nil {
		fmt.Printf("username '%s' does not exist in database\n", cmd.args[0])
		return fmt.Errorf("login command database select query error: %v\n", err)
	}
	s.config.SetUser(cmd.args[0])
	fmt.Printf("current user has been set to '%s'\n", cmd.args[0])
	return nil
}

func handlerregister(s *state, cmd command) error {
	var (
		ct       context.Context = context.Background()
		dbParams database.CreateUserParams
		dbUser   database.User
		err      error
	)
	if len(cmd.args) == 0 {
		fmt.Println("register command requires username")
		return fmt.Errorf("register command requires username\n")
	}
	// Check for existing user in database
	dbUser, err = s.db.GetUser(ct, cmd.args[0])
	if err == nil && dbUser.Name == cmd.args[0] {
		fmt.Printf("username '%s' already exists in database\n", dbUser.Name)
		return fmt.Errorf("username '%s' already exists in database\n", dbUser.Name)
	}
	// if err != nil {
	// 	return fmt.Errorf("register command database select query error: %v\n", err)
	// }
	// Create new user in database
	dbParams.ID = uuid.New()
	dbParams.CreatedAt = time.Now()
	dbParams.UpdatedAt = dbParams.CreatedAt
	dbParams.Name = cmd.args[0]
	dbUser, err = s.db.CreateUser(ct, dbParams)
	if err != nil {
		return fmt.Errorf("register command database insert query error: %v\n", err)
	}
	fmt.Println("User database record:")
	fmt.Printf("\tID = %v\n", dbUser.ID)
	fmt.Printf("\tCreated At = %v\n", dbUser.CreatedAt)
	fmt.Printf("\tUpdated At = %v\n", dbUser.UpdatedAt)
	fmt.Printf("\tName = %s\n", dbUser.Name)
	s.config.SetUser(dbUser.Name)
	fmt.Printf("username '%s' registered and set as current user\n", dbUser.Name)
	return nil
}

func handlerusers(s *state, cmd command) error {
	var (
		ct      context.Context = context.Background()
		dbUsers []database.User
		err     error
	)
	if len(cmd.args) > 0 {
		fmt.Println("users command requires no arguments")
		return fmt.Errorf("users command requires no arguments\n")
	}
	// Get all users in database
	dbUsers, err = s.db.GetUsers(ct)
	if err != nil {
		return fmt.Errorf("users command database select query error: %v\n", err)
	}
	for _, user := range dbUsers {
		fmt.Printf("* %s", user.Name)
		if user.Name == s.config.UserName {
			fmt.Printf(" (current)")
		}
		fmt.Println()
	}
	return nil
}

func handleragg(s *state, cmd command) error {
	var (
		ct  context.Context = context.Background()
		err error
		// feed *RSSFeed
	)
	if len(cmd.args) > 0 {
		fmt.Println("agg command requires feed URL")
		return fmt.Errorf("agg command requires feed URL\n")
	}
	_, err = fetchFeed(ct, "https://www.wagslane.dev/index.xml")
	if err != nil {
		fmt.Println("Error fetching feed")
		return fmt.Errorf("error %v fetching feed\n", err)
	}
	return nil
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	var (
		body    []byte
		client  http.Client
		req     *http.Request
		resp    *http.Response
		rssFeed RSSFeed
		err     error
	)
	req, err = http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gator")
	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// fmt.Printf("Status: %s\n", resp.Status)
	if resp.StatusCode > 299 {
		fmt.Printf("Unable to fetch Feed %s\n", feedURL)
		return nil, err
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// fmt.Println(string(body))
	err = xml.Unmarshal(body, &rssFeed)
	if err != nil {
		fmt.Printf("Error unmarshaling feed XML")
		return nil, err
	}
	// convert XML escaped entities into normal entities
	rssFeed.Channel.Title = html.UnescapeString(rssFeed.Channel.Title)
	// fmt.Printf("Feed Title: %s\n", rssFeed.Channel.Title)
	// fmt.Printf("Feed URL: %s\n", rssFeed.Channel.Link)
	rssFeed.Channel.Description = html.UnescapeString(rssFeed.Channel.Description)
	// fmt.Printf("Feed Description: %s\n", rssFeed.Channel.Description)
	// fmt.Println("Items:")
	for _, v := range rssFeed.Channel.Item {
		v.Title = html.UnescapeString(v.Title)
		// fmt.Printf("\tTitle: %s\n", v.Title)
		// fmt.Printf("\tURL: %s\n", v.Link)
		v.PubDate = html.UnescapeString(v.PubDate)
		// fmt.Printf("\tPublication Date: %s\n", v.PubDate)
		v.Description = html.UnescapeString(v.Description)
		// fmt.Printf("\tDescription: %s\n", v.Description)
		break
	}
	fmt.Println(rssFeed)
	return &rssFeed, nil
}

func main() {
	// Read configuration from file and create application state
	cfg := config.Read()
	as := new(state)
	as.config = &cfg

	// Open connection to database
	db, err := sql.Open("postgres", cfg.DbURL)
	dbQueries := database.New(db)
	as.db = dbQueries

	// Create commands structure and initialize map of handler functions
	ch := new(commands)
	ch.handler = make(map[string]func(*state, command) error, 0)
	ch.register("reset", handlerreset)
	ch.register("login", handlerlogin)
	ch.register("register", handlerregister)
	ch.register("users", handlerusers)
	ch.register("agg", handleragg)

	if len(os.Args) < 2 {
		fmt.Println("Insufficient arguments provided")
		os.Exit(1)
	}

	cmd := new(command)
	cmd.name = os.Args[1]
	cmd.args = os.Args[2:]
	err = ch.run(as, *cmd)
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
