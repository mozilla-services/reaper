package events

// AutoscalerConfig is the configuration for a Autoscaler
type AutoscalerConfig struct {
	EventReporterConfig
}

// Autoscaler is an EventReporter that tags AWS Resources
type Autoscaler struct {
	Config *AutoscalerConfig
}

func (e *Autoscaler) SetDryRun(b bool) {
	e.Config.DryRun = b
}

func NewAutoscaler(c *AutoscalerConfig) *Autoscaler {
	c.Name = "Autoscaler"
	return &Autoscaler{c}
}

func (e *Autoscaler) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	return nil
}

func (e *Autoscaler) NewStatistic(name string, value float64, tags []string) error {
	return nil
}

func (e *Autoscaler) NewCountStatistic(name string, tags []string) error {
	return nil
}

type Scaler interface {
	IsScaledDown() bool
	ScaleDown() error
	ScaleUp() error
}

func (e *Autoscaler) NewReapableEvent(r Reapable) error {
	if e.Config.ShouldTriggerFor(r) {
		if a, ok := r.(Scaler); ok && a.IsScaledDown() {
			// if it is scaled down, try to scale up
			return a.ScaleUp()
		} else if ok {
			// if it isn't scaled down, try to scale down
			return a.ScaleDown()
		}
	}
	return nil
}

func (e *Autoscaler) NewBatchReapableEvent(rs []Reapable) error {
	return nil
}
