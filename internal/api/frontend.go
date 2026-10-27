package api

// This file holds where the interface is served from and where the whole
// application is mounted. They are kept together because both answer "where
// is this deployment?" and both are read by the admin area; they have nothing
// to do with the endpoints in the files next to them.

// BasePath is the URL prefix the application is mounted under, so a redirect
// issued by a handler lands inside the mount point rather than at the site
// root.
func (s *Server) BasePath() string { return s.basePath }

// SetBasePath records the mount prefix.
func (s *Server) SetBasePath(prefix string) { s.basePath = prefix }

// SetFrontendSource records where the interface is being served from, so the
// admin area can say whether a frontend_dir change is in effect.
func (s *Server) SetFrontendSource(source string) { s.frontendSource = source }

// FrontendSource reports "embedded" or "directory".
func (s *Server) FrontendSource() string {
	if s.frontendSource == "" {
		return "embedded"
	}
	return s.frontendSource
}

// SetStaleFrontendVersion records the version of a custom interface built for
// a different release than this binary.
func (s *Server) SetStaleFrontendVersion(v string) { s.staleFrontend = v }

// StaleFrontendVersion returns that interface's version, or "".
func (s *Server) StaleFrontendVersion() string { return s.staleFrontend }
