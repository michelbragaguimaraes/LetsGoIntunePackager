package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/michelbragaguimaraes/LetsGoIntunePackager/internal/packager"
	"github.com/michelbragaguimaraes/LetsGoIntunePackager/internal/tui"
)

var (
	// Version info (set from main)
	version   = "dev"
	buildTime = "unknown"

	// CLI flags
	contentPath string
	setupFile   string
	outputPath  string
	quietMode   bool
)

// SetVersionInfo sets the version information from main
func SetVersionInfo(v, bt string) {
	version = v
	buildTime = bt
}

var rootCmd = &cobra.Command{
	Use:   "intunewin",
	Short: "Package installers for Microsoft Intune",
	Long: `LetsGoIntunePackager - A cross-platform CLI tool to create .intunewin packages
for Microsoft Intune Win32 app deployment.

This tool packages MSI/EXE installers into the encrypted .intunewin format
required by Microsoft Intune for Win32 app deployment.

Examples:
  # Interactive mode (default)
  intunewin

  # Quiet mode for CI/CD automation
  intunewin -c /path/to/source -s setup.msi -o /path/to/output -q`,
	Version: version,
	RunE: func(cmd *cobra.Command, args []string) error {
		if quietMode {
			return runQuietMode()
		}
		return runTUI()
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&contentPath, "content", "c", "", "Source folder containing the setup file")
	rootCmd.Flags().StringVarP(&setupFile, "setup", "s", "", "Setup file name (e.g., setup.msi or install.exe)")
	rootCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output folder for the .intunewin file")
	rootCmd.Flags().BoolVarP(&quietMode, "quiet", "q", false, "Quiet mode - no interactive UI, just process and exit")

	// Custom version template
	rootCmd.SetVersionTemplate(fmt.Sprintf("LetsGoIntunePackager version %s (built %s)\n", version, buildTime))
}

func runQuietMode() error {
	// Validate required flags in quiet mode
	if contentPath == "" {
		return fmt.Errorf("--content (-c) is required in quiet mode")
	}
	if setupFile == "" {
		return fmt.Errorf("--setup (-s) is required in quiet mode")
	}
	if outputPath == "" {
		return fmt.Errorf("--output (-o) is required in quiet mode")
	}

	// Validate paths exist
	if _, err := os.Stat(contentPath); os.IsNotExist(err) {
		return fmt.Errorf("source folder does not exist: %s", contentPath)
	}

	setupPath := fmt.Sprintf("%s/%s", contentPath, setupFile)
	if _, err := os.Stat(setupPath); os.IsNotExist(err) {
		return fmt.Errorf("setup file not found: %s", setupPath)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Println("Starting packaging process...")
	fmt.Printf("  Source: %s\n", contentPath)
	fmt.Printf("  Setup:  %s\n", setupFile)
	fmt.Printf("  Output: %s\n", outputPath)
	fmt.Println()

	// Call packager with progress callback
	result, err := packager.Package(contentPath, setupFile, outputPath, func(step string, pct float64) {
		fmt.Printf("  [%3.0f%%] %s\n", pct*100, step)
	})
	if err != nil {
		return fmt.Errorf("packaging failed: %w", err)
	}

	// Print results
	fmt.Println()
	fmt.Println("Package created successfully!")
	fmt.Printf("  Output:     %s\n", result.OutputPath)
	fmt.Printf("  Files:      %d\n", result.FileCount)
	fmt.Printf("  Source:     %s\n", packager.FormatSize(result.SourceSize))
	fmt.Printf("  Final size: %s\n", packager.FormatSize(result.FinalSize))

	return nil
}

func runTUI() error {
	// Check if flags were provided - if so, pass them as presets to TUI
	presets := &tui.Presets{
		ContentPath: contentPath,
		SetupFile:   setupFile,
		OutputPath:  outputPath,
	}

	// Run the TUI
	return tui.Run(presets)
}
