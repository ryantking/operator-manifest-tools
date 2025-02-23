package pinning

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"strings"

	"github.com/operator-framework/operator-manifest-tools/internal/utils"
	"github.com/operator-framework/operator-manifest-tools/pkg/imagename"
	"github.com/operator-framework/operator-manifest-tools/pkg/pullspec"
	"github.com/spf13/cobra"
)

type replaceCmdArgs struct {
	replacementFile utils.InputParam
	dryRun          bool
}

var (
	replaceCmdData = replaceCmdArgs{
		replacementFile: utils.NewInputParam(false),
	}
)

// replaceCmd represents the replace command
var replaceCmd = &cobra.Command{
	Use:   "replace [flags] MANIFEST_DIR REPLACEMENTS_FILES",
	Short: `Modify the image references in the CSVs found in the MANIFEST_DIR based on the given REPLACEMENTS_FILE.`,
	Args:  cobra.ExactArgs(2),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if err := utils.CheckIfDirectoryExists(args[0]); err != nil {
			return err
		}

		replaceCmdData.replacementFile.Name = args[1]
		return replaceCmdData.replacementFile.Init(cmd, args)
	},
	PostRunE: func(cmd *cobra.Command, args []string) error {
		return replaceCmdData.replacementFile.Close()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if replaceCmdData.dryRun {
			log.SetOutput(cmd.ErrOrStderr())
		}

		manifestDir := args[0]

		return replace(manifestDir, &replaceCmdData.replacementFile)
	},
}

func init() {
	// 	replaceCmdData.replacementFile.AddFlag(replaceCmd,
	// 		"replacements_file", "-", `The path to the REPLACEMENTS_FILE.
	// The format of this file is a simple JSON object
	// where each attribute is a string representing the original image reference and the
	// value is a string representing the new value for the image reference. Use - to
	// specify stdin.`)

	replaceCmd.Flags().BoolVar(&replaceCmdData.dryRun,
		"dry-run", false, strings.ReplaceAll(`When set, replacements are not performed. This is useful to determine if the CSV is
in a state that accepts replacements. By default this option is not set.`, "\n", " "))

}

// replace will read manifests from the directory and replace the images from
// the replacements directory.
func replace(manifestDir string, replacementsReader io.Reader) error {
	replacementsData, err := io.ReadAll(replacementsReader)
	if err != nil {
		return errors.New("failed to read data: " + err.Error())
	}

	var replacements map[string]string

	err = json.Unmarshal(replacementsData, &replacements)

	if err != nil {
		return errors.New("failed to replacement json: " + err.Error())
	}

	replacementImages := map[imagename.ImageName]imagename.ImageName{}

	for k, v := range replacements {
		key := imagename.Parse(k)
		value := imagename.Parse(v)

		if key == nil || value == nil {
			return errors.New("failed to parse replacement images: " + err.Error())
		}
		replacementImages[*key] = *value
	}

	operatorManifests, err := pullspec.FromDirectory(manifestDir, pullspec.DefaultHeuristic)

	if err != nil {
		return err
	}

	for _, manifest := range operatorManifests {
		err := manifest.ReplacePullSpecsEverywhere(replacementImages)

		if err != nil {
			return errors.New("failed to replace everywhere: " + err.Error())
		}

		err = manifest.SetRelatedImages()

		if err != nil {
			return errors.New("failed to set related images: " + err.Error())
		}

		if replaceCmdData.dryRun {
			log.Println("dryRun is enabled, no output was generated")
			continue
		}

		err = manifest.Dump(nil)
		if err != nil {
			return errors.New("failed to update the manifests: " + err.Error())
		}
	}

	return nil

}
