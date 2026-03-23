package command

import (
	"fmt"
	"strings"
)

func HandlePrivmsg(ctx *Context) {
	handleMessage(ctx, "PRIVMSG")
}

func HandleNotice(ctx *Context) {
	handleMessage(ctx, "NOTICE")
}

func handleMessage(ctx *Context, cmd string) {
	if len(ctx.Message.Params) == 0 {
		ctx.reply("411", ctx.Client.Nick(), fmt.Sprintf("No recipient given (%s)", cmd))
		return
	}
	if len(ctx.Message.Params) < 2 {
		ctx.reply("412", ctx.Client.Nick(), "No text to send")
		return
	}

	target := ctx.Message.Params[0]
	text := ctx.Message.Params[1]
	line := fmt.Sprintf(":%s %s %s :%s\r\n", ctx.Client.Prefix(), cmd, target, text)

	if strings.HasPrefix(target, "#") || strings.HasPrefix(target, "&") {
		if !ctx.Server.BroadcastToChannel(ctx.Client, target, line) {
			ctx.reply("403", ctx.Client.Nick(), target, "No such channel")
		}
	} else {
		recipient := ctx.Server.FindClientByNick(target)
		if recipient == nil {
			ctx.reply("401", ctx.Client.Nick(), target, "No such nick/channel")
			return
		}
		recipient.SendLine(line)
	}
}
