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
	"strconv"
	"time"

	"github.com/dragonicorn/gator/internal/config"
	"github.com/dragonicorn/gator/internal/database"

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
		ctx context.Context = context.Background()
		err error
	)
	if len(cmd.args) > 0 {
		fmt.Printf("%s command requires no arguments\n", cmd.name)
		return fmt.Errorf("%s command requires no arguments\n", cmd.name)
	}
	// Delete all users from database
	err = s.db.DeleteUsers(ctx)
	if err != nil {
		fmt.Println("Unable to remove all users from database")
		return fmt.Errorf("%s command database delete query error: %v\n", cmd.name, err)
	}
	fmt.Println("All users have been removed from database")
	return nil
}

func handlerlogin(s *state, cmd command) error {
	var (
		ctx context.Context = context.Background()
		err error
	)
	if len(cmd.args) == 0 {
		fmt.Printf("%s command requires username\n", cmd.name)
		return fmt.Errorf("%s command requires username\n", cmd.name)
	}
	// Check for existing user in database
	_, err = s.db.GetUser(ctx, cmd.args[0])
	if err != nil {
		fmt.Printf("username '%s' does not exist in database\n", cmd.args[0])
		return fmt.Errorf("%s command database select query error: %v\n", cmd.name, err)
	}
	s.config.SetUser(cmd.args[0])
	fmt.Printf("current user has been set to '%s'\n", cmd.args[0])
	return nil
}

func handlerregister(s *state, cmd command) error {
	var (
		ctx      context.Context = context.Background()
		dbParams database.CreateUserParams
		dbUser   database.User
		err      error
	)
	if len(cmd.args) == 0 {
		fmt.Printf("%s command requires username\n", cmd.name)
		return fmt.Errorf("%s command requires username\n", cmd.name)
	}
	// Check for existing user in database
	dbUser, err = s.db.GetUser(ctx, cmd.args[0])
	if err == nil && dbUser.Name == cmd.args[0] {
		fmt.Printf("username '%s' already exists in database\n", dbUser.Name)
		return fmt.Errorf("username '%s' already exists in database\n", dbUser.Name)
	}
	// if err != nil {
	// 	return fmt.Errorf("%s command database select query error: %v\n", cmd.name, err)
	// }
	// Create new user in database
	dbParams.ID = uuid.New()
	dbParams.CreatedAt = time.Now()
	dbParams.UpdatedAt = dbParams.CreatedAt
	dbParams.Name = cmd.args[0]
	dbUser, err = s.db.CreateUser(ctx, dbParams)
	if err != nil {
		return fmt.Errorf("%s command database insert query error: %v\n", cmd.name, err)
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
		ctx     context.Context = context.Background()
		dbUsers []database.User
		err     error
	)
	if len(cmd.args) > 0 {
		fmt.Printf("%s command requires no arguments\n", cmd.name)
		return fmt.Errorf("%s command requires no arguments\n", cmd.name)
	}
	// Get all users in database
	dbUsers, err = s.db.GetUsers(ctx)
	if err != nil {
		return fmt.Errorf("%s command database select query error: %v\n", cmd.name, err)
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
		// ctx context.Context = context.Background()
		err error
		// feed *RSSFeed
		ticker *time.Ticker
	)
	if len(cmd.args) == 0 {
		fmt.Printf("%s command requires feed update interval\n", cmd.name)
		return fmt.Errorf("%s command requires feed update interval\n", cmd.name)
	}
	interval, err := time.ParseDuration(cmd.args[0])
	// _, err = fetchFeed(ctx, "https://www.wagslane.dev/index.xml")
	if err != nil {
		fmt.Println("Error determining feed update interval")
		return fmt.Errorf("error %v determining feed update interval\n", err)
	}
	fmt.Printf("Collecting feeds every %s\n", interval)
	ticker = time.NewTicker(interval)
	for ; ; <-ticker.C {
		scrapeFeeds(s)
	}
	return nil
}

func scrapeFeeds(s *state) error {
	var (
		ctx          context.Context = context.Background()
		dbFeed       database.Feed
		dbParams     database.MarkFeedFetchedParams
		dbPost       database.Post
		dbPostParams database.CreatePostParams
		err          error
		pd           time.Time
		rssFeed      *RSSFeed
		rssItem      RSSItem
	)
	dbFeed, err = s.db.GetNextFeedToFetch(ctx)
	if err != nil {
		fmt.Printf("Unable to get next feed to fetch from database\n")
		return fmt.Errorf("Unable to get next feed to fetch from database\n")
	}
	fmt.Printf("  Scraping feed %s...\n", dbFeed.Name)
	dbParams.ID = dbFeed.ID
	dbParams.UpdatedAt = time.Now()
	err = s.db.MarkFeedFetched(ctx, dbParams)
	if err != nil {
		return fmt.Errorf("mark feed fetched database update query error: %v\n", err)
	}
	rssFeed, err = fetchFeed(ctx, dbFeed.Url)
	if err != nil {
		return fmt.Errorf("error %v fetching feed\n", err)
	}
	for _, rssItem = range rssFeed.Channel.Item {
		// Create new post in database
		dbPostParams.ID = uuid.New()
		dbPostParams.CreatedAt = time.Now()
		dbPostParams.UpdatedAt = dbPostParams.CreatedAt
		dbPostParams.Title.String = rssItem.Title
		dbPostParams.Title.Valid = true
		dbPostParams.Url = rssItem.Link
		dbPostParams.Description.String = html.UnescapeString(rssItem.Description)
		dbPostParams.Description.Valid = true
		// fmt.Printf("PubDate = %s\n", rssItem.PubDate)
		pd, err = time.Parse("Mon, 02 Jan 2006 15:04:05 -0700", rssItem.PubDate)
		if err != nil {
			fmt.Printf("rssItem.PubDate parsing error: %v\n", err)
			return fmt.Errorf("rssItem.PubDate parsing error: %v\n", err)
		}
		// fmt.Printf("pd/PublishedAt = %v\n", pd)
		dbPostParams.PublishedAt = pd
		dbPostParams.FeedID = dbFeed.ID
		dbPost, err = s.db.CreatePost(ctx, dbPostParams)
		if err != nil {
			if err.Error() == "pq: duplicate key value violates unique constraint \"posts_url_key\"" {
				fmt.Printf("ignoring rssItem url already in database...\n")
				continue
			}
			fmt.Printf("rssItem database insert query error: %v\n", err)
			return fmt.Errorf("rssItem database insert query error: %v\n", err)
		}
		fmt.Println("Post database record added to database:")
		fmt.Printf("\tID = %v\n", dbPost.ID)
		fmt.Printf("\tCreated At = %v\n", dbPost.CreatedAt)
		fmt.Printf("\tUpdated At = %v\n", dbPost.UpdatedAt)
		if dbPost.Title.Valid {
			fmt.Printf("\tTitle = %s\n", dbPost.Title.String)
		}
		fmt.Printf("\tUrl = %s\n", dbPost.Url)
		if dbPost.Description.Valid {
			fmt.Printf("\tDescription = %s\n", dbPost.Description.String)
		}
		fmt.Printf("\tPublication Date = %s\n", dbPost.PublishedAt)
		fmt.Printf("\tFeed ID = %s\n", dbPost.FeedID)
		// break
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

func getURL(ctx context.Context, url string) ([]byte, error) {
	var (
		body   []byte
		client http.Client
		req    *http.Request
		resp   *http.Response
		err    error
	)
	req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
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
		fmt.Printf("Unable to fetch Feed %s\n", url)
		return nil, err
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	var (
		body []byte
		// client  http.Client
		// req     *http.Request
		// resp    *http.Response
		rssFeed RSSFeed
		err     error
	)
	// req, err = http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	// if err != nil {
	// 	return nil, err
	// }
	// req.Header.Set("User-Agent", "gator")
	// resp, err = client.Do(req)
	// if err != nil {
	// 	return nil, err
	// }
	// defer resp.Body.Close()
	// // fmt.Printf("Status: %s\n", resp.Status)
	// if resp.StatusCode > 299 {
	// 	fmt.Printf("Unable to fetch Feed %s\n", feedURL)
	// 	return nil, err
	// }
	// body, err = io.ReadAll(resp.Body)
	// if err != nil {
	// 	return nil, err
	// }
	body, err = getURL(ctx, feedURL)
	if err != nil {
		fmt.Printf("Error getting URL")
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
	// fmt.Println(rssFeed)
	return &rssFeed, nil
}

func handlerbrowse(s *state, cmd command, user database.User) error {
	var (
		ctx              context.Context = context.Background()
		dbGetPostsParams database.GetPostsForUserParams
		dbPosts          []database.GetPostsForUserRow
		dbPost           database.GetPostsForUserRow
		limit            int = 2
		// post             []byte
		err error
	)
	if len(cmd.args) == 0 {
		fmt.Printf("%s command allows optional limit to posts returned\n", cmd.name)
	} else if len(cmd.args) == 1 {
		limit, err = strconv.Atoi(cmd.args[0])
		if err != nil {
			return fmt.Errorf("%s command requires integer limit value\n", cmd.name)
		}
	} else {
		return fmt.Errorf("%s command only allows optional limit argument\n", cmd.name)
	}
	fmt.Printf("%s command will limit posts returned to %d\n", cmd.name, limit)

	// Get most recent posts (up to limit) in database from all feeds followed by current user
	dbGetPostsParams.Name = s.config.UserName
	dbGetPostsParams.Limit = int32(limit)
	dbPosts, err = s.db.GetPostsForUser(ctx, dbGetPostsParams)
	if err != nil {
		return fmt.Errorf("%s command database select query error: %v\n", cmd.name, err)
	}
	for _, dbPost = range dbPosts {
		fmt.Println("Post database record retrieved:")
		fmt.Printf("\tID = %v\n", dbPost.ID)
		fmt.Printf("\tCreated At = %v\n", dbPost.CreatedAt)
		fmt.Printf("\tUpdated At = %v\n", dbPost.UpdatedAt)
		if dbPost.Title.Valid {
			fmt.Printf("\tTitle = %s\n", dbPost.Title.String)
		}
		fmt.Printf("\tUrl = %s\n", dbPost.Url)
		if dbPost.Description.Valid {
			fmt.Printf("\tDescription = %s\n", dbPost.Description.String)
		}
		fmt.Printf("\tPublication Date = %s\n", dbPost.PublishedAt)
		fmt.Printf("\tFeed ID = %s\n", dbPost.FeedID)
		fmt.Println()

		_, err = getURL(ctx, dbPost.Url)
		// _post, err = getURL(ctx, dbPost.Url)
		if err != nil {
			return fmt.Errorf("Error getting post %s\n", dbPost.Url)
		}
		// fmt.Println(len(string(post)))
	}

	return nil
}

func handleraddfeed(s *state, cmd command, user database.User) error {
	var (
		ctx          context.Context = context.Background()
		dbParams     database.CreateFeedParams
		dbFeed       database.Feed
		dbFFParams   database.CreateFeedFollowParams
		dbFeedFollow database.CreateFeedFollowRow
		err          error
	)
	if len(cmd.args) < 2 {
		fmt.Printf("%s command requires feed name and URL\n", cmd.name)
		return fmt.Errorf("%s command requires feed name and URL\n", cmd.name)
	}
	// Check for existing feed in database
	dbFeed, err = s.db.GetFeed(ctx, cmd.args[0])
	if err == nil && dbFeed.Name == cmd.args[0] {
		fmt.Printf("feed '%s' already exists in database\n", dbFeed.Name)
		return fmt.Errorf("feed '%s' already exists in database\n", dbFeed.Name)
	}
	// if err != nil {
	// 	return fmt.Errorf("addfeed command database select query error: %v\n", err)
	// }

	// Create new feed in database
	dbParams.ID = uuid.New()
	dbParams.CreatedAt = time.Now()
	dbParams.UpdatedAt = dbParams.CreatedAt
	dbParams.Name = cmd.args[0]
	dbParams.Url = cmd.args[1]
	dbParams.UserID = user.ID
	dbFeed, err = s.db.CreateFeed(ctx, dbParams)
	if err != nil {
		return fmt.Errorf("%s command database insert query error: %v\n", cmd.name, err)
	}

	fmt.Println("Feed database record:")
	fmt.Printf("\tID = %v\n", dbFeed.ID)
	fmt.Printf("\tCreated At = %v\n", dbFeed.CreatedAt)
	fmt.Printf("\tUpdated At = %v\n", dbFeed.UpdatedAt)
	fmt.Printf("\tName = %s\n", dbFeed.Name)
	fmt.Printf("\tURL = %s\n", dbFeed.Url)
	fmt.Printf("\tUserID = %v\n", dbFeed.UserID)
	fmt.Printf("feed '%s' added\n\n", dbFeed.Name)

	// Create new feedfollow in database
	dbFFParams.ID = uuid.New()
	dbFFParams.CreatedAt = time.Now()
	dbFFParams.UpdatedAt = dbParams.CreatedAt
	dbFFParams.UserID = user.ID
	dbFFParams.FeedID = dbFeed.ID
	dbFeedFollow, err = s.db.CreateFeedFollow(ctx, dbFFParams)
	if err != nil {
		return fmt.Errorf("%s command database insert query error: %v\n", cmd.name, err)
	}

	fmt.Println("FeedFollow database record:")
	fmt.Printf("\tID = %v\n", dbFeedFollow.ID)
	fmt.Printf("\tCreated At = %v\n", dbFeedFollow.CreatedAt)
	fmt.Printf("\tUpdated At = %v\n", dbFeedFollow.UpdatedAt)
	fmt.Printf("\tUserID = %v\n", dbFeedFollow.UserID)
	fmt.Printf("\tFeedID = %v\n", dbFeedFollow.FeedID)
	fmt.Printf("feed '%s' followed by '%s'\n", dbFeedFollow.FeedName, dbFeedFollow.UserName)
	return nil
}

func handlerfeeds(s *state, cmd command) error {
	var (
		ctx     context.Context = context.Background()
		dbFeeds []database.Feed
		dbUser  database.User
		err     error
	)
	if len(cmd.args) > 0 {
		fmt.Printf("%s command requires no arguments\n", cmd.name)
		return fmt.Errorf("%s command requires no arguments\n", cmd.name)
	}
	// Get all feeds in database
	dbFeeds, err = s.db.GetFeeds(ctx)
	if err != nil {
		return fmt.Errorf("%s command database select query error: %v\n", cmd.name, err)
	}
	for _, feed := range dbFeeds {
		fmt.Printf("* %s\n", feed.Name)
		fmt.Printf("* %s\n", feed.Url)

		// Get current user from database
		dbUser, err = s.db.GetUserById(ctx, feed.UserID)
		if err != nil {
			return fmt.Errorf("%s command database select query error: %v\n", cmd.name, err)
		}
		fmt.Printf("* %s\n", dbUser.Name)
		fmt.Println()
	}
	return nil
}

func handlerfollow(s *state, cmd command, user database.User) error {
	var (
		ctx          context.Context = context.Background()
		dbParams     database.CreateFeedFollowParams
		dbFeedFollow database.CreateFeedFollowRow
		dbFeed       database.Feed
		err          error
	)
	if len(cmd.args) < 1 {
		fmt.Printf("%s command requires feed URL\n", cmd.name)
		return fmt.Errorf("%s command requires feed URL\n", cmd.name)
	}
	// Get feed by URL
	dbFeed, err = s.db.GetFeedByURL(ctx, cmd.args[0])
	if err != nil {
		return fmt.Errorf("%s command database select query error: %v\n", cmd.name, err)
	}

	// Create new feedfollow in database
	dbParams.ID = uuid.New()
	dbParams.CreatedAt = time.Now()
	dbParams.UpdatedAt = dbParams.CreatedAt
	dbParams.UserID = user.ID
	dbParams.FeedID = dbFeed.ID
	dbFeedFollow, err = s.db.CreateFeedFollow(ctx, dbParams)
	if err != nil {
		return fmt.Errorf("%s command database insert query error: %v\n", cmd.name, err)
	}

	fmt.Println("FeedFollow database record:")
	fmt.Printf("\tID = %v\n", dbFeedFollow.ID)
	fmt.Printf("\tCreated At = %v\n", dbFeedFollow.CreatedAt)
	fmt.Printf("\tUpdated At = %v\n", dbFeedFollow.UpdatedAt)
	fmt.Printf("\tUserID = %v\n", dbFeedFollow.UserID)
	fmt.Printf("\tFeedID = %v\n", dbFeedFollow.FeedID)
	fmt.Printf("feed '%s' followed by '%s'\n", dbFeedFollow.FeedName, dbFeedFollow.UserName)
	return nil
}

func handlerfollowing(s *state, cmd command) error {
	var (
		ctx           context.Context = context.Background()
		dbFeedFollows []database.GetFeedFollowsForUserRow
		err           error
	)
	if len(cmd.args) > 0 {
		fmt.Printf("%s command requires no arguments\n", cmd.name)
		return fmt.Errorf("%s command requires no arguments\n", cmd.name)
	}
	// Get all feeds in database followed by current user
	dbFeedFollows, err = s.db.GetFeedFollowsForUser(ctx, s.config.UserName)
	if err != nil {
		return fmt.Errorf("%s command database select query error: %v\n", cmd.name, err)
	}
	for _, feed := range dbFeedFollows {
		fmt.Printf("* %s\n", feed.FeedName)
	}
	return nil
}

func handlerunfollow(s *state, cmd command, user database.User) error {
	var (
		ctx      context.Context = context.Background()
		dbParams database.DeleteFeedFollowParams
		err      error
	)
	if len(cmd.args) < 1 {
		fmt.Printf("%s command requires feed URL\n", cmd.name)
		return fmt.Errorf("%s command requires feed URL\n", cmd.name)
	}
	// Delete feedfollow from database
	dbParams.Name = user.Name
	dbParams.Url = cmd.args[0]
	err = s.db.DeleteFeedFollow(ctx, dbParams)
	if err != nil {
		fmt.Println("Unable to remove feed follow from database")
		return fmt.Errorf("%s command database delete query error: %v\n", cmd.name, err)
	}
	fmt.Printf("feed '%s' no longer followed by '%s'\n", cmd.args[0], user.Name)
	return nil
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		// Get current user from database
		user, err := s.db.GetUser(context.Background(), s.config.UserName)
		if err != nil {
			return err
		}
		return handler(s, cmd, user)
	}
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
	ch.register("addfeed", middlewareLoggedIn(handleraddfeed))
	ch.register("feeds", handlerfeeds)
	ch.register("follow", middlewareLoggedIn(handlerfollow))
	ch.register("following", handlerfollowing)
	ch.register("unfollow", middlewareLoggedIn(handlerunfollow))
	ch.register("agg", handleragg)
	ch.register("browse", middlewareLoggedIn(handlerbrowse))

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
	// os.Exit(0)
}
