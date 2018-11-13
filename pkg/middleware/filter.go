package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gobuffalo/buffalo"
	"github.com/gomods/athens/pkg/errors"
	"github.com/gomods/athens/pkg/module"
	"github.com/gomods/athens/pkg/paths"
)

// NewFilterMiddleware builds a middleware function that implements the
// filters configured in the filter file.
func NewFilterMiddleware(mf *module.Filter, upstreamEndpoint string) buffalo.MiddlewareFunc {
	const op errors.Op = "actions.NewFilterMiddleware"

	return func(next buffalo.Handler) buffalo.Handler {
		return func(c buffalo.Context) error {
			mod, err := paths.GetModule(c)

			if err != nil {
				// if there is no module the path we are hitting is not one related to modules, like /
				return next(c)
			}

			rule := mf.Rule(mod)
			switch rule {
			case module.Exclude:
				// Exclude: ignore request for this module
				return c.Render(http.StatusForbidden, nil)
			case module.Include:
				// Include: please handle this module in a usual way
				return next(c)
			case module.Direct:
				// Direct: do not store modules locally, use upstream proxy
				newURL := redirectToUpstreamURL(upstreamEndpoint, c.Request().URL)
				return c.Redirect(http.StatusSeeOther, newURL)
			}

			return next(c)
		}
	}
}

func redirectToUpstreamURL(upstreamEndpoint string, u *url.URL) string {
	return strings.TrimSuffix(upstreamEndpoint, "/") + u.Path
}
