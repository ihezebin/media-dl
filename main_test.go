package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandTree(t *testing.T) {
	root := newRootCommand()
	video := findCommand(root, "video")
	music := findCommand(root, "music")
	if video == nil || music == nil {
		t.Fatalf("root command must expose video and music domains")
	}
	if findCommand(root, "info") != nil || findCommand(root, "download") != nil {
		t.Fatalf("video commands must not remain at the root")
	}

	for _, name := range []string{"info", "download"} {
		if findCommand(video, name) == nil {
			t.Fatalf("video command missing %q", name)
		}
	}
	for _, name := range []string{"search", "download"} {
		if findCommand(music, name) == nil {
			t.Fatalf("music command missing %q", name)
		}
	}

	if !strings.HasPrefix(findCommand(video, "info").Use, "info ") {
		t.Fatalf("video info command has unexpected Use: %q", findCommand(video, "info").Use)
	}
	if !strings.HasPrefix(findCommand(music, "search").Use, "search ") {
		t.Fatalf("music search command has unexpected Use: %q", findCommand(music, "search").Use)
	}
}

func TestCommonFlagsAreAvailableToBothDomains(t *testing.T) {
	root := newRootCommand()
	for _, path := range [][]string{
		{"video", "info"},
		{"video", "download"},
		{"music", "search"},
		{"music", "download"},
	} {
		command, _, err := root.Find(path)
		if err != nil {
			t.Fatalf("find command %v: %v", path, err)
		}
		for _, name := range []string{"proxy", "cookie", "cookies"} {
			if command.Flag(name) == nil {
				t.Fatalf("command %v does not inherit --%s", path, name)
			}
		}
	}
}

func findCommand(parent *cobra.Command, name string) *cobra.Command {
	for _, command := range parent.Commands() {
		if command.Name() == name {
			return command
		}
	}
	return nil
}
