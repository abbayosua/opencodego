package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/opencode-go/opencode/internal/storage"
)

func sessionCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: opencode session <list|read> [id]\n")
		os.Exit(1)
	}

	store := openStore()
	defer store.Close()

	switch args[0] {
	case "list":
		sessionList(store)

	case "read":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: opencode session read <id>\n")
			os.Exit(1)
		}
		sessionRead(store, args[1])

	default:
		fmt.Fprintf(os.Stderr, "Unknown session subcommand: %s\n", args[0])
		fmt.Fprintf(os.Stderr, "Usage: opencode session <list|read> [id]\n")
		os.Exit(1)
	}
}

func sessionList(store storage.Store) {
	ctx := context.Background()
	sessions, err := store.ListSessions(ctx, storage.ListSessionsFilter{Limit: 50})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing sessions: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tTitle\tModel\tMessages\tCreated")

	for _, s := range sessions {
		ctx := context.Background()
		allMsgs, err := store.ListMessages(ctx, storage.ListMessagesFilter{SessionID: s.ID, Limit: 1000})
		msgCount := 0
		if err == nil {
			msgCount = len(allMsgs)
		}

		title := truncateStr(s.Title, 40)
		created := s.CreatedAt.Format("02 Jan 15:04")
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", s.ID, title, s.Model, msgCount, created)
	}
	w.Flush()
}

func sessionRead(store storage.Store, id string) {
	sess, msgs, err := store.GetSessionWithMessages(context.Background(), id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading session: %v\n", err)
		os.Exit(1)
	}
	if sess == nil {
		fmt.Fprintf(os.Stderr, "Session not found: %s\n", id)
		os.Exit(1)
	}

	fmt.Printf("Session: %s\n", sess.ID)
	fmt.Printf("Title:   %s\n", sess.Title)
	fmt.Printf("Model:   %s\n", sess.Model)
	fmt.Printf("Agent:   %s\n", sess.Agent)
	fmt.Printf("Created: %s\n", sess.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated: %s\n", sess.UpdatedAt.Format("2006-01-02 15:04:05"))
	if sess.Summary != "" {
		fmt.Printf("Summary: %s\n", sess.Summary)
	}

	fmt.Println("\nMessages:")
	for i, m := range msgs {
		fmt.Printf("\n%d. [%s] %s\n", i+1, m.Role, m.ID[:min(20, len(m.ID))])
		if m.Content != "" {
			content := m.Content
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			// Remove internal tool call markers for clean display
			content = cleanContent(content)
			fmt.Printf("   %s\n", content)
		}
	}
}

func openStore() storage.Store {
	dbPath := os.Getenv("OPENCODE_DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join(os.TempDir(), "opencode.db")
	}
	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	return store
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func cleanContent(s string) string {
	if len(s) > 0 && s[0] == '\n' {
		s = s[1:]
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
