package main

import (
	"context"
	"database/sql"
	"fmt"
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
