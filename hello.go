onService != nil {
		return c.introspectionService
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return introspection.NewIntrospectionServiceFromClient(introspectionapi.NewIntrospectionClient(c.conn))
}

// LeasesService returns the underlying Leases Client
func (c *Client) LeasesService() leases.Manager {
	if c.leasesService != nil {
		return c.leasesService
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return leasesproxy.NewLeaseManager(leasesapi.NewLeasesClient(c.conn))
}

// HealthService returns the underlying GRPC HealthClient
func (c *Client) HealthService() grpc_health_v1.HealthClient {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return grpc_health_v1.NewHealthClient(c.conn)
}

// EventService returns the underlying event service
func (c *Client) EventService() EventService {
	if c.eventService != nil {
		return c.eventService
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return NewEventServiceFromClient(eventsapi.NewEventsClient(c.conn))
}

// SandboxStore returns the underlying sandbox store client
func (c *Client) SandboxStore() sandbox.Store {
	if c.sandboxStore != nil {
		return c.sandboxStore
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return NewRemoteSandboxStore(sandboxsapi.NewStoreClient(c.conn))
}

// SandboxController returns the underlying sandbox controller client
func (c *Client) SandboxController() sandbox.Controller {
	if c.sandboxController != nil {
		return c.sandboxController
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return NewSandboxRemoteController(sandboxsapi.NewControllerClient(c.conn))
}

// VersionService returns the underlying VersionClient
func (c *Client) VersionService() versionservice.VersionClient {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return versionservice.NewVersionClient(c.conn)
}

// Conn returns the underlying GRPC connection object
func (c *Client) Conn() *grpc.ClientConn {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return c.conn
}

// Version of containerd
type Version struct {
	// Version number
	Version string
	// Revision from git that was built
	Revision string
}

// Version returns the version of containerd that the client is connected to
func (c *Client) Version(ctx context.Context) (Version, error) {
	c.connMu.Lock()
	if c.conn == nil {
		c.connMu.Unlock()
		return Version{}, fmt.Errorf("no grpc connection available: %w", errdefs.ErrUnavailable)
	}
	c.connMu.Unlock()
	response, err := c.VersionService().Version(ctx, &ptypes.Empty{})
	if err != nil {
		return Version{}, err
	}
	return Version{
		Version:  response.Version,
		Revision: response.Revision,
	}, nil
}

// ServerInfo represents the introspected server information
type ServerInfo struct {
	UUID string
}

// Server returns server information from the introspection service
func (c *Client) Server(ctx context.Context) (ServerInfo, error) {
	c.connMu.Lock()
	if c.conn == nil {
		c.connMu.Unlock()
		return ServerInfo{}, fmt.Errorf("no grpc connection available: %w", errdefs.ErrUnavailable)
	}
	c.connMu.Unlock()

	response, err := c.IntrospectionService().Server(ctx, &ptypes.Empty{})
	if err != nil {
		return ServerInfo{}, err
	}
	return ServerInfo{
		UUID: response.UUID,
	}, nil
}

func (c *Client) resolveSnapshotterName(ctx context.Context, name string) (string, error) {
	if name == "" {
		label, err := c.GetLabel(ctx, defaults.DefaultSnapshotterNSLabel)
		if err != nil {
			return "", err
		}

		if label != "" {
			name = label
		} else {
			name = DefaultSnapshotter
		}
	}

	return name, nil
}

func (c *Client) getSnapshotter(ctx context.Context, name string) (snapshots.Snapshotter, error) {
	name, err := c.resolveSnapshotterName(ctx, name)
	if err != nil {
		return nil, err
	}

	s := c.SnapshotService(name)
	if s == nil {
		return nil, fmt.Errorf("snapshotter %s was not found: %w", name, errdefs.ErrNotFound)
	}

	return s, nil
}

// CheckRuntime returns true if the current runtime matches the expected
// runtime. Providing various parts of the runtime schema will match those
// parts of the expected runtime
func CheckRuntime(current, expected string) bool {
	cp := strings.Split(current, ".")
	l := len(cp)
	for i, p := range strings.Split(expected, ".") {
		if i > l {
			return false
		}
		if p != cp[i] {
			return false
		}
	}
	return true
}

// GetSnapshotterSupportedPlatforms returns a platform matchers which represents the
// supported platforms for the given snapshotters
func (c *Client) GetSnapshotterSupportedPlatforms(ctx context.Context, snapshotterName string) (platforms.MatchComparer, error) {
	filters := []string{fmt.Sprintf("type==%s, id==%s", plugin.SnapshotPlugin, snapshotterName)}
	in := c.IntrospectionService()

	resp, err := in.Plugins(ctx, filters)
	if err != nil {
		return nil, err
	}

	if len(resp.Plugins) <= 0 {
		return nil, fmt.Errorf("inspection service could not find snapshotter %s plugin", snapshotterName)
	}

	sn := resp.Plugins[0]
	snPlatforms := toPlatforms(sn.Platforms)
	return platforms.Any(snPlatforms...), nil
}

func toPlatforms(pt []*apitypes.Platform) []ocispec.Platform {
	platforms := make([]ocispec.Platform, len(pt))
	for i, p := range pt {
		platforms[i] = ocispec.Platform{
			Architecture: p.Architecture,
			OS:           p.OS,
			Variant:      p.Variant,
		}
	}
	return platforms
}