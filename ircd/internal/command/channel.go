package command

import (
	"fmt"
	"strings"
)

func HandleJoin(ctx *Context) {
	if len(ctx.Message.Params) == 0 {
		ctx.reply("461", ctx.Client.Nick(), "JOIN", "Not enough parameters")
		return
	}
	channels := strings.Split(ctx.Message.Params[0], ",")
	for _, ch := range channels {
		ch = strings.TrimSpace(ch)
		if !strings.HasPrefix(ch, "#") && !strings.HasPrefix(ch, "&") {
			ctx.reply("403", ctx.Client.Nick(), ch, "No such channel")
			continue
		}
		ctx.Server.JoinChannel(ctx.Client, ch)
	}
}

func HandlePart(ctx *Context) {
	if len(ctx.Message.Params) == 0 {
		ctx.reply("461", ctx.Client.Nick(), "PART", "Not enough parameters")
		return
	}
	reason := ""
	if len(ctx.Message.Params) > 1 {
		reason = ctx.Message.Params[1]
	}
	channels := strings.Split(ctx.Message.Params[0], ",")
	for _, ch := range channels {
		ch = strings.TrimSpace(ch)
		ctx.Server.PartChannel(ctx.Client, ch, reason)
	}
}

func HandleTopic(ctx *Context) {
	if len(ctx.Message.Params) == 0 {
		ctx.reply("461", ctx.Client.Nick(), "TOPIC", "Not enough parameters")
		return
	}
	channelName := ctx.Message.Params[0]
	if len(ctx.Message.Params) == 1 {
		topic := ctx.Server.ChannelTopic(channelName)
		if topic == "" {
			ctx.reply("331", ctx.Client.Nick(), channelName, "No topic is set")
		} else {
			ctx.reply("332", ctx.Client.Nick(), channelName, topic)
		}
		return
	}
	ctx.Server.SetChannelTopic(ctx.Client, channelName, ctx.Message.Params[1])
}

func HandleNames(ctx *Context) {
	if len(ctx.Message.Params) == 0 {
		ctx.reply("461", ctx.Client.Nick(), "NAMES", "Not enough parameters")
		return
	}
	channelName := ctx.Message.Params[0]
	names := ctx.Server.ChannelNames(channelName)
	if names != nil {
		ctx.reply("353", ctx.Client.Nick(), "=", channelName, strings.Join(names, " "))
	}
	ctx.reply("366", ctx.Client.Nick(), channelName, "End of /NAMES list")
}

func HandleList(ctx *Context) {
	ctx.reply("321", ctx.Client.Nick(), "Channel", "Users  Name")
	for _, ch := range ctx.Server.ListChannels() {
		ctx.Client.SendLine(fmt.Sprintf(":%s 322 %s %s %d :%s\r\n",
			ctx.Server.ServerName(), ctx.Client.Nick(),
			ch.Name, ch.MemberCount, ch.Topic))
	}
	ctx.reply("323", ctx.Client.Nick(), "End of /LIST")
}
