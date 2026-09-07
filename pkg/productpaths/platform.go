package productpaths

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/agentstation/starmap/pkg/errors"
)

// Product names a supported application namespace.
type Product string

const (
	// Starmap owns standalone catalog application paths.
	Starmap Product = "starmap"
	// Starport owns gateway application paths.
	Starport Product = "starport"
)

// UserDefaults returns a native user-directory lookup for the selected product.
// This explicit host adapter reads platform directory inputs only when called.
func UserDefaults(product Product) Lookup {
	return func(root Root) (string, error) {
		if product != Starmap && product != Starport {
			return "", &errors.ValidationError{Field: "paths.product", Message: "must be starmap or starport"}
		}
		var base string
		var err error
		switch runtime.GOOS {
		case "linux":
			switch root {
			case Config:
				base, err = xdgDirectory("XDG_CONFIG_HOME", ".config")
			case Data:
				base, err = xdgDirectory("XDG_DATA_HOME", filepath.Join(".local", "share"))
			case State:
				base, err = xdgDirectory("XDG_STATE_HOME", filepath.Join(".local", "state"))
			case Cache:
				base, err = xdgDirectory("XDG_CACHE_HOME", ".cache")
			default:
				return "", invalidRoot(root)
			}
		case "darwin":
			switch root {
			case Config, Data, State:
				base, err = os.UserConfigDir()
				if err == nil {
					return filepath.Join(base, string(product), string(root)), nil
				}
			case Cache:
				base, err = os.UserCacheDir()
			default:
				return "", invalidRoot(root)
			}
		case "windows":
			switch root {
			case Config:
				base, err = os.UserConfigDir()
			case Data, State, Cache:
				base, err = os.UserCacheDir()
				if err == nil {
					return filepath.Join(base, string(product), string(root)), nil
				}
			default:
				return "", invalidRoot(root)
			}
		default:
			return "", &errors.ConfigError{Component: "product paths", Message: "the platform requires explicit application roots"}
		}
		if err != nil {
			return "", &errors.ConfigError{Component: "product paths", Message: "cannot resolve the native user directory", Err: err}
		}
		return filepath.Join(base, string(product)), nil
	}
}

func xdgDirectory(name, fallback string) (string, error) {
	if value, present := os.LookupEnv(name); present && value != "" {
		if !filepath.IsAbs(value) {
			return "", &errors.ValidationError{Field: name, Message: "must be an absolute directory"}
		}
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, fallback), nil
}

func invalidRoot(root Root) error {
	return &errors.ValidationError{Field: string(root), Message: "is not an application root"}
}
