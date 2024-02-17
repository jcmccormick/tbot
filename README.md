# TBot 
Go-based Twitch IRC bot

# Requirements

1. Twitch Developer Application
    - OAuth Redirect URL `http://localhost:7776/auth/callback`
2. Environment Variables
    - TWITCH_CLIENT_ID
    - TWITCH_CLIENT_SECRET
    - TWITCH_USER

# Install

1. Clone the repo
2. `go run tbot.go` to start the program

# About

This tool will connect to the given TWITCH_USER's IRC chatroom. Standard input
can be given in the console once connected. Upon first connection, you must
click the URLs provided in the console, and login with your Twitch account via
the browser.
