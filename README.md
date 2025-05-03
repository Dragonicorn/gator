# GATOR - A Simple Blog Aggregator

gator is a command-line program to aggregate content from RSS feeds written in Go for the guided project 'Build a Blog Aggregator in Go' course at Boot.dev.

In order to use this program the Go language runtime and Postgres SQL will need to be installed. Information can be found here (https://go.dev/doc/install) and here (https://www.postgresql.org/download/).

Once the prerequisite software is installed, gator can be installed from the command line by downloading it from github using the command 'go install github.com/dragonicorn/gator@latest'.

A **'.gatorconfig.json'** file must be created in the user's home directory (e.g. in linux **'~/.gatorconfig.json'**) with the following content:

`
{
  "db_url":"postgres://postgres:postgres@localhost:5432/gator?sslmode=disable"
}
`

---

Once the application is ready to go, run it using 'gator cmd _option_' where cmd is one of the following:

    * reset - initialize the database, clearing out any previous content (use this before any of the other commands)
	* register _username_ - add a user to the database and set them as the active user
	* login - set a previously registered user as the active user
	* users - display a list of registered users
    * addfeed _url_ - register a RSS feed url as a source of content posts
	* feeds - display a list of registered RSS feeds
	* follow _url_ - add a registered feed to the active user's list of followed feeds
	* following - display a list of the active user's followed feeds
	* unfollow _url_ - remove a feed from the active user's list of followed feeds
	* agg _interval_ - update each registered feed with a list of content posts every 'interval' (e.g. agg 60s)
	* browse _limit_ - display a list of 'limit' most recent posts from all followed feeds for the active user. (If no limit is specified, it will default to 2).

---

No guarantees on how it will perform as only limited alpha testing has been performed on this primarily educational project.