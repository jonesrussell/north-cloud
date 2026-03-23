package command

import (
	"fmt"
	"strings"
)

func HandleNick(ctx *Context) {
	if len(ctx.Message.Params) == 0 {
		ctx.reply("431", ctx.clientNick(), "No nickname given")
		return
	}
	nick := ctx.Message.Params[0]
	if !isValidNick(nick) {
		ctx.reply("432", ctx.clientNick(), nick, "Erroneous nickname")
		return
	}
	if existing := ctx.Server.FindClientByNick(nick); existing != nil && existing != ctx.Client {
		ctx.reply("433", ctx.clientNick(), nick, "Nickname is already in use")
		return
	}
	wasRegistered := ctx.Client.Registered()
	if err := ctx.Server.ChangeNick(ctx.Client, nick); err != nil {
		ctx.reply("433", ctx.clientNick(), nick, "Nickname is already in use")
		return
	}
	if !wasRegistered && ctx.Client.Registered() {
		sendWelcome(ctx)
	}
}

func HandleUser(ctx *Context) {
	if ctx.Client.Registered() {
		ctx.reply("462", ctx.Client.Nick(), "You may not reregister")
		return
	}
	if len(ctx.Message.Params) < 4 {
		ctx.reply("461", ctx.clientNick(), "USER", "Not enough parameters")
		return
	}
	username := ctx.Message.Params[0]
	realname := ctx.Message.Params[3]
	ctx.Client.SetUser(username, realname)
	if ctx.Client.Registered() {
		sendWelcome(ctx)
	}
}

func HandlePass(ctx *Context) {
	if ctx.Client.Registered() {
		ctx.reply("462", ctx.Client.Nick(), "You may not reregister")
		return
	}
}

func HandleQuit(ctx *Context) {
	reason := "Client quit"
	if len(ctx.Message.Params) > 0 {
		reason = ctx.Message.Params[0]
	}
	ctx.Client.SendLine(fmt.Sprintf("ERROR :Closing link: %s (%s)\r\n", ctx.Client.Hostname(), reason))
	ctx.Server.UnregisterClient(ctx.Client)
}

func HandlePing(ctx *Context) {
	token := ctx.Server.ServerName()
	if len(ctx.Message.Params) > 0 {
		token = ctx.Message.Params[0]
	}
	ctx.Client.SendLine(fmt.Sprintf(":%s PONG %s :%s\r\n", ctx.Server.ServerName(), ctx.Server.ServerName(), token))
}

func HandlePong(_ *Context) {}

func sendWelcome(ctx *Context) {
	nick := ctx.Client.Nick()
	sn := ctx.Server.ServerName()
	net := ctx.Server.NetworkName()

	ctx.reply("001", nick, fmt.Sprintf("Welcome to the %s IRC Network %s", net, ctx.Client.Prefix()))
	ctx.reply("002", nick, fmt.Sprintf("Your host is %s, running NorthCloud IRCd v0.1.0", sn))
	ctx.reply("003", nick, "This server was created recently")
	ctx.reply("004", nick, sn, "NorthCloudIRCd-0.1.0", "io", "otn")
	ctx.reply("005", nick, "CHANTYPES=#", "CHANMODES=,,,nt", "PREFIX=(o)@", fmt.Sprintf("NETWORK=%s", net), "are supported by this server")

	motd := ctx.Server.MOTD()
	if motd != "" {
		ctx.reply("375", nick, fmt.Sprintf("- %s Message of the day -", sn))
		for _, line := range strings.Split(motd, "\n") {
			if line != "" {
				ctx.reply("372", nick, fmt.Sprintf("- %s", line))
			}
		}
		ctx.reply("376", nick, "End of /MOTD command")
	}

	ctx.reply("251", nick, fmt.Sprintf("There are %d users on 1 server", ctx.Server.ClientCount()))
}

func (ctx *Context) reply(numeric string, params ...string) {
	msg := fmt.Sprintf(":%s %s", ctx.Server.ServerName(), numeric)
	for i, p := range params {
		if i == len(params)-1 && strings.Contains(p, " ") {
			msg += " :" + p
		} else {
			msg += " " + p
		}
	}
	msg += "\r\n"
	ctx.Client.SendLine(msg)
}

func (ctx *Context) clientNick() string {
	nick := ctx.Client.Nick()
	if nick == "" {
		return "*"
	}
	return nick
}

func isValidNick(nick string) bool {
	if nick == "" || len(nick) > 30 {
		return false
	}
	if nick[0] >= '0' && nick[0] <= '9' {
		return false
	}
	for _, r := range nick {
		if r == ' ' || r == ',' || r == '*' || r == '?' || r == '!' || r == '@' || r == '#' || r == '&' || r == ':' {
			return false
		}
	}
	return true
}
