package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewCompletionCmd returns the `aura completion <shell>` command.
func NewCompletionCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "completion <shell>",
		Short: "Generate shell completion script (bash, zsh, fish)",
		Long: `Generate a shell completion script and source it to enable tab completion.

  bash:   source <(aura completion bash)
  zsh:    source <(aura completion zsh)
  fish:   aura completion fish | source`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(os.Stdout)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			default:
				return fmt.Errorf("unsupported shell %q: use bash, zsh, or fish", args[0])
			}
		},
	}
}
