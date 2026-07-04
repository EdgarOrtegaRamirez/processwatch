// ProcessWatch - A lightweight cross-platform process supervisor
//
// Usage:
//
//	processwatch start [config.yml]     Start all processes
//	processwatch stop                   Stop all processes
//	processwatch restart <name>         Restart a specific process
//	processwatch status                 Show status of all processes
//	processwatch status --json          Show status as JSON
//	processwatch init                   Generate sample config
//	processwatch version                Show version
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/EdgarOrtegaRamirez/processwatch/internal/config"
	"github.com/EdgarOrtegaRamirez/processwatch/internal/manager"
	"github.com/EdgarOrtegaRamirez/processwatch/internal/process"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "start":
		cmdStart(os.Args[2:])
	case "stop":
		cmdStop()
	case "restart":
		cmdRestart(os.Args[2:])
	case "status":
		cmdStatus(os.Args[2:])
	case "init":
		cmdInit()
	case "version":
		fmt.Printf("processwatch %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`ProcessWatch - Lightweight Cross-Platform Process Supervisor

USAGE:
    processwatch <command> [options]

COMMANDS:
    start [config.yml]    Start all processes (default: processwatch.yml)
    stop                  Stop all running processes
    restart <name>        Restart a specific process by name
    status [--json]       Show status of all processes
    init                  Generate a sample configuration file
    version               Show version
    help                  Show this help

EXAMPLES:
    processwatch start                    # Start with default config
    processwatch start myconfig.yml       # Start with custom config
    processwatch status --json            # JSON status output
    processwatch restart web-server       # Restart specific process
    processwatch stop                     # Stop all processes

CONFIGURATION:
    ProcessWatch uses YAML configuration files. Run 'processwatch init'
    to generate a sample configuration.

    Programs are supervised with configurable:
    - Auto-restart with exponential backoff
    - Log capture and rotation
    - Health checks (HTTP, TCP)
    - Signal forwarding
    - Environment variable injection`)
}

func cmdStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output status as JSON")
	configFile := fs.String("config", "", "Config file path")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: processwatch start [options] [config.yml]")
		fs.PrintDefaults()
	}

	// Parse flags, allow positional arg for config file
	var positional []string
	for i := 1; i < len(args); i++ {
		if args[i] == "--json" || args[i] == "-j" {
			*jsonOutput = true
		} else if args[i] == "--config" || args[i] == "-c" {
			if i+1 < len(args) {
				*configFile = args[i+1]
				i++
			}
		} else if !strings.HasPrefix(args[i], "-") {
			positional = append(positional, args[i])
		}
	}

	// Config file from positional arg
	cfgPath := *configFile
	if cfgPath == "" {
		if len(positional) > 0 {
			cfgPath = positional[0]
		} else {
			cfgPath = findConfigFile()
		}
	}

	if cfgPath == "" {
		fmt.Fprintln(os.Stderr, "Error: No config file found. Create one with 'processwatch init'")
		os.Exit(1)
	}

	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	mgr, err := manager.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating manager: %v\n", err)
		os.Exit(1)
	}
	defer mgr.Close()

	if err := mgr.StartAll(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting processes: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		// Wait a moment for processes to start, then output JSON
		mgr.WaitForExit()
		data, _ := mgr.StatusJSON()
		fmt.Println(string(data))
	} else {
		// Wait for processes to exit
		mgr.WaitForExit()
	}
}

func cmdStop() {
	fmt.Println("Stopping all processes...")

	// Try to find and load config to get process names
	cfgPath := findConfigFile()
	if cfgPath == "" {
		fmt.Println("No config file found. Nothing to stop.")
		return
	}

	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	mgr, err := manager.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer mgr.Close()

	mgr.StopAll()
}

func cmdRestart(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: processwatch restart <process-name>")
		os.Exit(1)
	}

	name := args[0]

	cfgPath := findConfigFile()
	if cfgPath == "" {
		fmt.Fprintln(os.Stderr, "Error: No config file found")
		os.Exit(1)
	}

	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	mgr, err := manager.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer mgr.Close()

	if err := mgr.StartAll(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Wait for processes to be running
	for _, info := range mgr.Status() {
		if info.State != process.StateRunning {
			fmt.Fprintf(os.Stderr, "Process %s is not running (state: %s)\n", name, info.State)
		}
	}

	if err := mgr.Restart(name); err != nil {
		fmt.Fprintf(os.Stderr, "Error restarting %s: %v\n", name, err)
		os.Exit(1)
	}

	fmt.Printf("Restarted process: %s\n", name)
}

func cmdStatus(args []string) {
	jsonOutput := false
	for _, arg := range args {
		if arg == "--json" || arg == "-j" {
			jsonOutput = true
		}
	}

	cfgPath := findConfigFile()
	if cfgPath == "" {
		fmt.Println("No config file found. Run 'processwatch init' to create one.")
		return
	}

	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	mgr, err := manager.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer mgr.Close()

	if jsonOutput {
		data, err := mgr.StatusJSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	} else {
		fmt.Println(mgr.StatusText())
	}
}

func cmdInit() {
	outFile := "processwatch.yml"
	if _, err := os.Stat(outFile); err == nil {
		fmt.Printf("File %s already exists. Overwrite? [y/N]: ", outFile)
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Aborted.")
			return
		}
	}

	if err := os.WriteFile(outFile, []byte(config.SampleConfig()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		os.Exit(1)
	}

	absPath, _ := filepath.Abs(outFile)
	fmt.Printf("Created sample config: %s\n", absPath)
	fmt.Println("\nEdit the file to configure your processes, then run:")
	fmt.Println("  processwatch start")
}

func findConfigFile() string {
	// Check common config file names
	candidates := []string{
		"processwatch.yml",
		"processwatch.yaml",
		"processwatch.json",
		".processwatch.yml",
		".processwatch.yaml",
		"supervisor.yml",
		"supervisor.yaml",
	}

	for _, name := range candidates {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}

	return ""
}
