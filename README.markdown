# cares #

Cares is a microblog application written in the Go language. It supports posting to a single microblog on your site.

Readers can subscribe to your feed, while readers with supported software can subscribe to immediate notification of your posts. (Cares currently supports [RSS cloud][], for readers using [River2][].)

Cares' name is inspired by another Go microblog application, [nobodycares][].

[RSS cloud]: http://walkthrough.rsscloud.org/
[River2]: http://river2.newsriver.org/
[nobodycares]: http://code.google.com/p/nobodycares/


## Requirements ##

* Go 1
* PostgreSQL
* a web server


## Installing ##

Cares is a web application that runs on your server, so it's probably a little fiddly for most people.

Install Cares to your Go environment with `go get`. (You might want to make a new environment manually or with [gowork][] first.)

	$ go get github.com/markpasc/cares

Set up a PostgreSQL database, and run Cares to initialize the database and make your user account:

	$ createdb cares
	$ createuser cares
	$ psql -c 'grant all privileges on database cares to cares' cares
	$ cares --database 'dbname=cares user=cares' --init

Cares will ask for a login name and password for you to use when using the site, and set up the database. Then Cares is ready to run. You can check by invoking `cares` manually and connecting directly on its port:

	$ cares --database 'dbname=cares user=cares' --port 8080

Yay, now Cares is built and running. Open it in the browser to mess around.

[gowork]: https://github.com/markpasc/gowork


## Usage ##

Once installed and running, your site will appear on the web. To post, go to the home page and type `p`. A new post will appear that you can type text into. Type `return` to make the post. (The web site will ask for the username and password you entered when you installed Cares.) To cancel the post, press the `escape` key instead (or leave the page).

Customize your site by editing the HTML templates (in the `html/` directory) and the static web files (in the `static/` directory) as appropriate.


## Really installing ##

* Use a tool like [Supervisor][] to keep the server running. See `extras/supervisor.example.conf` for an example Supervisor configuration.
* Run the Cares app as an “app server,” behind a web server like Nginx. See `extras/nginx.example.conf` for an example Nginx site configuration.
* Have the web server serve over HTTPS instead of HTTP.

[supervisor]: http://supervisord.org/


## Future enhancements ##

* following and reading others' streams
* Atom & [PubSubHubbub][]
* [JSON Activity Streams][]
* as much of [tent.io][] as is feasible

[PubSubHubbub]: https://code.google.com/p/pubsubhubbub/
[JSON Activity Streams]: http://activitystrea.ms/specs/json/1.0/
[tent.io]: http://tent.io/
