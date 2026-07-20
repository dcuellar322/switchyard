package cli

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	teamApplication "switchyard.dev/switchyard/internal/team/application"
	teamDomain "switchyard.dev/switchyard/internal/team/domain"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func newTeamCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "team", Short: "Manage signed portable team configuration"}
	command.AddCommand(
		newTeamPublisherCommand(options), newTeamKeyCommand(options), newTeamBundleCommand(options),
		newTeamTemplateCommand(options), newTeamPolicyCommand(options), newTeamRegistryCommand(options), newTeamSyncCommand(options),
	)
	return command
}

func newTeamPublisherCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "publisher", Short: "Review trusted Ed25519 publisher identities"}
	command.AddCommand(
		&cobra.Command{Use: "list", Short: "List trusted public signing keys", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
			client, err := daemonClient(command.Context(), options)
			if err != nil {
				return err
			}
			items, err := client.TeamPublishers(command.Context())
			if err != nil {
				return err
			}
			return writeResult(options, "team.publisher.list", items, func(writer io.Writer) error {
				rows := make([][]string, 0, len(items))
				for _, item := range items {
					rows = append(rows, []string{item.Id, item.Name, item.TrustedAt.Format(time.RFC3339)})
				}
				if len(rows) == 0 {
					_, err := fmt.Fprintln(writer, "No team publishers trusted.")
					return err
				}
				return humanList(writer, []string{"PUBLISHER", "NAME", "TRUSTED"}, rows)
			})
		}},
		newTeamPublisherTrustCommand(options),
	)
	return command
}

func newTeamPublisherTrustCommand(options *rootOptions) *cobra.Command {
	name, publicKey := "", ""
	yes := false
	command := &cobra.Command{Use: "trust", Short: "Trust one exact public signing key", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		if !yes {
			return usageError("TEAM_CONFIRMATION_REQUIRED", "publisher trust requires --yes after reviewing the exact public key")
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		publisher, err := client.TrustTeamPublisher(command.Context(), generated.TeamPublisherTrustRequest{Name: name, PublicKey: publicKey, ConfirmRisk: yes})
		if err != nil {
			return err
		}
		return writeResult(options, "team.publisher.trust", publisher, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s trusted as %s\n", publisher.Name, publisher.Id)
			return err
		})
	}}
	command.Flags().StringVar(&name, "name", "", "publisher display name")
	command.Flags().StringVar(&publicKey, "public-key", "", "base64 Ed25519 public key")
	command.Flags().BoolVar(&yes, "yes", false, "confirm the reviewed signing identity")
	_ = command.MarkFlagRequired("name")
	_ = command.MarkFlagRequired("public-key")
	return command
}

func newTeamKeyCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "key", Short: "Create offline bundle signing keys"}
	command.AddCommand(newTeamKeyGenerateCommand(options), newTeamKeyShowCommand(options))
	return command
}

func newTeamKeyGenerateCommand(options *rootOptions) *cobra.Command {
	output := ""
	command := &cobra.Command{Use: "generate", Short: "Generate a private Ed25519 bundle signing key", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error {
		publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		key := signingKeyFile{SchemaVersion: signingKeySchema, PublisherID: teamApplication.PublisherID(publicKey), PublicKey: base64.StdEncoding.EncodeToString(publicKey), PrivateKey: base64.StdEncoding.EncodeToString(privateKey)}
		//nolint:gosec // G117: this explicit key-generation command writes the private key to a new owner-only file.
		encoded, err := json.MarshalIndent(key, "", "  ")
		if err != nil {
			return err
		}
		if err := writeExclusiveFile(output, append(encoded, '\n'), 0o600); err != nil {
			return err
		}
		return writeResult(options, "team.key.generate", map[string]string{"publisherId": key.PublisherID, "publicKey": key.PublicKey, "path": output}, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "Private key: %s\nPublisher: %s\nPublic key: %s\n", output, key.PublisherID, key.PublicKey)
			return err
		})
	}}
	command.Flags().StringVar(&output, "output", "", "new owner-only signing key file")
	_ = command.MarkFlagRequired("output")
	return command
}

func newTeamKeyShowCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "show <key-file>", Short: "Print only a signing key's public identity", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		key, _, err := readSigningKey(args[0])
		if err != nil {
			return err
		}
		return writeResult(options, "team.key.show", map[string]string{"publisherId": key.PublisherID, "publicKey": key.PublicKey}, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "Publisher: %s\nPublic key: %s\n", key.PublisherID, key.PublicKey)
			return err
		})
	}}
}

func newTeamBundleCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "bundle", Short: "Sign, verify, and install portable configuration bundles"}
	command.AddCommand(newTeamBundleListCommand(options), newTeamBundleSignCommand(options), newTeamBundleInstallCommand(options))
	return command
}

func newTeamBundleListCommand(options *rootOptions) *cobra.Command {
	kind := ""
	command := &cobra.Command{Use: "list", Short: "List installed verified bundles", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		items, err := client.TeamBundles(command.Context(), kind)
		if err != nil {
			return err
		}
		return writeResult(options, "team.bundle.list", items, func(writer io.Writer) error {
			if len(items) == 0 {
				_, err := fmt.Fprintln(writer, "No signed team bundles installed.")
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{item.Metadata.Id, string(item.Kind), item.Metadata.Version, item.Metadata.PublisherId})
			}
			return humanList(writer, []string{"BUNDLE", "KIND", "VERSION", "PUBLISHER"}, rows)
		})
	}}
	command.Flags().StringVar(&kind, "kind", "", "optional bundle kind filter")
	return command
}

func newTeamBundleSignCommand(options *rootOptions) *cobra.Command {
	name, version, payloadPath, keyPath, output, expires := "", "", "", "", "", ""
	command := &cobra.Command{Use: "sign <kind> <bundle-id>", Short: "Create a canonical Ed25519-signed configuration bundle", Args: cobra.ExactArgs(2), RunE: func(_ *cobra.Command, args []string) error {
		payload, err := readBoundedFile(payloadPath, 2<<20)
		if err != nil {
			return err
		}
		_, privateKey, err := readSigningKey(keyPath)
		if err != nil {
			return err
		}
		metadata := teamDomain.BundleMetadata{ID: args[1], Name: name, Version: version, CreatedAt: time.Now().UTC()}
		if expires != "" {
			parsed, err := time.Parse(time.RFC3339, expires)
			if err != nil {
				return usageError("BUNDLE_EXPIRY_INVALID", "--expires must use RFC3339")
			}
			metadata.ExpiresAt = &parsed
		}
		bundle, err := teamApplication.SignBundle(teamDomain.Bundle{Kind: teamDomain.BundleKind(args[0]), Metadata: metadata, Payload: payload}, privateKey)
		if err != nil {
			return err
		}
		encoded, err := json.MarshalIndent(bundle, "", "  ")
		if err != nil {
			return err
		}
		if err := writeExclusiveFile(output, append(encoded, '\n'), 0o644); err != nil {
			return err
		}
		return writeResult(options, "team.bundle.sign", bundle, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s signed by %s\noutput: %s\n", bundle.Metadata.ID, bundle.Metadata.PublisherID, output)
			return err
		})
	}}
	command.Flags().StringVar(&name, "name", "", "bundle display name")
	command.Flags().StringVar(&version, "version", "", "publisher-defined bundle version")
	command.Flags().StringVar(&payloadPath, "payload", "", "JSON payload file")
	command.Flags().StringVar(&keyPath, "key", "", "owner-only Ed25519 signing key file")
	command.Flags().StringVar(&output, "output", "", "new signed bundle file")
	command.Flags().StringVar(&expires, "expires", "", "optional RFC3339 expiration")
	for _, flag := range []string{"name", "version", "payload", "key", "output"} {
		_ = command.MarkFlagRequired(flag)
	}
	return command
}

func newTeamBundleInstallCommand(options *rootOptions) *cobra.Command {
	yes := false
	command := &cobra.Command{Use: "install <bundle-file>", Short: "Verify and install a bundle from a trusted publisher", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if !yes {
			return usageError("TEAM_CONFIRMATION_REQUIRED", "bundle installation requires --yes after reviewing kind, publisher, and payload")
		}
		encoded, err := readBoundedFile(args[0], 2<<20)
		if err != nil {
			return err
		}
		var bundle generated.TeamBundle
		if err := decodeStrictJSON(encoded, &bundle); err != nil {
			return usageError("BUNDLE_INVALID", err.Error())
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		installed, err := client.InstallTeamBundle(command.Context(), generated.TeamBundleInstallRequest{Bundle: bundle, ConfirmRisk: yes})
		if err != nil {
			return err
		}
		return writeResult(options, "team.bundle.install", installed, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "%s installed and signature verified\n", installed.Metadata.Id)
			return err
		})
	}}
	command.Flags().BoolVar(&yes, "yes", false, "confirm reviewed signed configuration")
	return command
}

func newTeamTemplateCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "template", Short: "Render signed portable project templates"}
	command.AddCommand(newTeamTemplateRenderCommand(options))
	return command
}

func newTeamTemplateRenderCommand(options *rootOptions) *cobra.Command {
	var values []string
	output := ""
	command := &cobra.Command{Use: "render <bundle>", Short: "Render and validate a signed project template", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		resolved := map[string]string{}
		for _, value := range values {
			key, item, ok := strings.Cut(value, "=")
			if !ok || key == "" {
				return usageError("TEMPLATE_VALUE_INVALID", "--set values use name=value")
			}
			resolved[key] = item
		}
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		manifest, err := client.RenderTeamTemplate(command.Context(), args[0], resolved)
		if err != nil {
			return err
		}
		if output != "" {
			encoded, err := json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				return err
			}
			if err := writeExclusiveFile(output, append(encoded, '\n'), 0o644); err != nil {
				return err
			}
		}
		return writeResult(options, "team.template.render", manifest, func(writer io.Writer) error {
			if output != "" {
				_, err := fmt.Fprintln(writer, output)
				return err
			}
			return writePrettyJSON(writer, manifest)
		})
	}}
	command.Flags().StringArrayVar(&values, "set", nil, "template value as name=value (repeatable)")
	command.Flags().StringVar(&output, "output", "", "optional new manifest JSON file")
	return command
}

func newTeamPolicyCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "policy", Short: "Read effective restrictive signed policy", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		policy, err := client.EffectiveTeamPolicy(command.Context())
		if err != nil {
			return err
		}
		return writeResult(options, "team.policy", policy, func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "remote capabilities: %s\nremote actions: %s\ntelemetry: %t\nsources: %s\n", strings.Join(policy.AllowedRemoteCapabilities, ","), strings.Join(policy.AllowedRemoteActions, ","), policy.TelemetryAllowed, strings.Join(policy.SourceBundleIds, ","))
			return err
		})
	}}
}

func newTeamRegistryCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "registry", Short: "List curated metadata from signed plugin registries", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		client, err := daemonClient(command.Context(), options)
		if err != nil {
			return err
		}
		items, err := client.CuratedPlugins(command.Context())
		if err != nil {
			return err
		}
		return writeResult(options, "team.registry", items, func(writer io.Writer) error {
			if len(items) == 0 {
				_, err := fmt.Fprintln(writer, "No signed plugin registry installed.")
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{item.Id, item.Version, item.Publisher, item.DownloadUrl})
			}
			return humanList(writer, []string{"PLUGIN", "VERSION", "PUBLISHER", "DOWNLOAD"}, rows)
		})
	}}
}
