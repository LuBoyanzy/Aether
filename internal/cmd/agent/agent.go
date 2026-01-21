package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"aether"
	"aether/agent"
	"aether/agent/health"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh"
)

// cli options
type cmdOptions struct {
	key    string // key is the public key(s) for SSH authentication.
	listen string // listen is the address or port to listen on.
	hubURL string // hubURL is the URL of the Aether hub.
	token  string // token is the token to use for authentication.
}

// parse parses the command line flags and populates the config struct.
// It returns true if a subcommand was handled and the program should exit.
func (opts *cmdOptions) parse() bool {
	subcommand := ""
	if len(os.Args) > 1 {
		subcommand = os.Args[1]
	}

	// Subcommands that don't require any pflag parsing
	switch subcommand {
	case "-v", "version":
		fmt.Println(aether.AppName+"-agent", aether.Version)
		return true
	case "health":
		err := health.Check()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print("ok")
		return true
	}

	// pflag.CommandLine.ParseErrorsWhitelist.UnknownFlags = true
	pflag.StringVarP(&opts.key, "key", "k", "", "Public key(s) for SSH authentication")
	pflag.StringVarP(&opts.listen, "listen", "l", "", "Address or port to listen on")
	pflag.StringVarP(&opts.hubURL, "url", "u", "", "URL of the Aether hub")
	pflag.StringVarP(&opts.token, "token", "t", "", "Token to use for authentication")
	chinaMirrors := pflag.BoolP("china-mirrors", "c", false, "Use mirror for update (gh.aether.dev) instead of GitHub")
	help := pflag.BoolP("help", "h", false, "Show this help message")

	// Convert old single-dash long flags to double-dash for backward compatibility
	flagsToConvert := []string{"key", "listen", "url", "token"}
	for i, arg := range os.Args {
		for _, flag := range flagsToConvert {
			singleDash := "-" + flag
			doubleDash := "--" + flag
			if arg == singleDash {
				os.Args[i] = doubleDash
				break
			} else if strings.HasPrefix(arg, singleDash+"=") {
				os.Args[i] = doubleDash + arg[len(singleDash):]
				break
			}
		}
	}

	pflag.Usage = func() {
		builder := strings.Builder{}
		builder.WriteString("Usage: ")
		builder.WriteString(os.Args[0])
		builder.WriteString(" [command] [flags]\n")
		builder.WriteString("\nCommands:\n")
		builder.WriteString("  health    Check if the agent is running\n")
		// builder.WriteString("  help      Display this help message\n")
		builder.WriteString("  update    Update to the latest version\n")
		builder.WriteString("\nFlags:\n")
		fmt.Print(builder.String())
		pflag.PrintDefaults()
	}

	// Parse all arguments with pflag
	pflag.Parse()

	// Must run after pflag.Parse()
	switch {
	case *help || subcommand == "help":
		pflag.Usage()
		return true
	case subcommand == "update":
		agent.Update(*chinaMirrors)
		return true
	}

	// Set environment variables from CLI flags (if provided)
	if opts.hubURL != "" {
		os.Setenv("HUB_URL", opts.hubURL)
	}
	if opts.token != "" {
		os.Setenv("TOKEN", opts.token)
	}
	return false
}

// loadPublicKeys loads the public keys from the command line flag, environment variable, or key file.
func (opts *cmdOptions) loadPublicKeys() ([]ssh.PublicKey, error) {
	// Try command line flag first
	if opts.key != "" {
		return agent.ParseKeys(opts.key)
	}

	// Try environment variable
	if key, ok := agent.GetEnv("KEY"); ok && key != "" {
		return agent.ParseKeys(key)
	}

	// Try key file
	keyFile, ok := agent.GetEnv("KEY_FILE")
	if !ok {
		return nil, fmt.Errorf("no key provided: must set -key flag, KEY env var, or KEY_FILE env var. Use 'aether-agent help' for usage")
	}

	pubKey, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}
	return agent.ParseKeys(string(pubKey))
}

func (opts *cmdOptions) getAddress() string {
	return agent.GetAddress(opts.listen)
}

func main() {
	// 允许“直接运行 ./aether-agent”时自动读取同目录 env 文件。
	// systemd 模式下 EnvironmentFile 已负责注入环境变量；这里不会覆盖非空的已存在变量。
	if exe, err := os.Executable(); err == nil {
		if err := loadEnvFileFromDir(filepath.Dir(exe)); err != nil {
			log.Fatal("Failed to load env file: ", err)
		}
	} else {
		log.Fatal("Failed to resolve executable path: ", err)
	}

	var opts cmdOptions
	subcommandHandled := opts.parse()

	if subcommandHandled {
		return
	}

	var serverConfig agent.ServerOptions
	var err error
	serverConfig.Keys, err = opts.loadPublicKeys()
	if err != nil {
		log.Fatal("Failed to load public keys:", err)
	}

	addr := opts.getAddress()
	serverConfig.Addr = addr
	serverConfig.Network = agent.GetNetwork(addr)

	a, err := agent.NewAgent()
	if err != nil {
		log.Fatal("Failed to create agent: ", err)
	}

	if err := a.Start(serverConfig); err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}
