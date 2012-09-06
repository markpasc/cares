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

Create a new Go environment (manually or with [gowork][]). In that environment, install Cares with `go get`:

	$ go get github.com/markpasc/cares

Set up a PostgreSQL database, and run Cares to initialize the database and make your user account:

	$ cares --database 'dbname=cares user=cares' --init

Cares will ask for a login name and password for you to use when using the site, and set up the database. Then Cares is ready to run. You can check by invoking `cares` manually and connecting directly on its port:

	$ cares --database 'dbname=cares user=cares' --port 8080

Use a tool like [Supervisor][] to keep the server running normally. Configure a web server (such as Nginx) to proxy to the Cares site if you like. This will allow you to serve static files directly instead of through the Cares app.

[gowork]: https://github.com/markpasc/gowork
[supervisor]: http://supervisord.org/


## Usage ##

Once installed and running, your site will appear on the web. To post, go to the home page and type `p`. A new post will appear that you can type text into. Type `return` to make the post. (The web site will ask for the username and password you entered when you installed Cares.) To cancel the post, press the `escape` key instead (or leave the page).

Customize your site by editing the HTML templates (in the `html/` directory) and the static web files (in the `static/` directory) as appropriate.


## Future enhancements ##

* following and reading others' streams
* Atom & [PubSubHubbub][]
* [JSON Activity Streams][]
* [tent.io?][]

[PubSubHubbub]: https://code.google.com/p/pubsubhubbub/
[JSON Activity Streams]: http://activitystrea.ms/specs/json/1.0/
[tent.io?]: http://tent.io/
