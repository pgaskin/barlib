package pulseaudio

type DevType int

const (
	SUBSCRIPTION_MASK_ALL DevType = 0x02ff
	SUBSCRIPTION_MASK_AUTOLOAD = 0x0100
	SUBSCRIPTION_MASK_CARD = 0x0200
	SUBSCRIPTION_MASK_CLIENT = 0x0020
	SUBSCRIPTION_MASK_MODULE = 0x0010
	SUBSCRIPTION_MASK_NULL = 0x0000
	SUBSCRIPTION_MASK_SAMPLE_CACHE = 0x0040
	SUBSCRIPTION_MASK_SERVER = 0x0080
	SUBSCRIPTION_MASK_SINK = 0x0001
	SUBSCRIPTION_MASK_SINK_INPUT = 0x0004
	SUBSCRIPTION_MASK_SOURCE = 0x0002
	SUBSCRIPTION_MASK_SOURCE_OUTPUT = 0x0008
)

// Updates returns a channel with PulseAudio updates.
func (c *Client) Updates() (updates <-chan struct{}, err error) {
	_, err = c.request(commandSubscribe, uint32Tag, uint32(SUBSCRIPTION_MASK_ALL))
	if err != nil {
		return nil, err
	}
	return c.updates, nil
}



func (c *Client) UpdatesByType(devType DevType) (updates <-chan struct{}, err error) {
	_, err = c.request(commandSubscribe, uint32Tag, uint32(devType))
	if err != nil {
		return nil, err
	}
	return c.updates, nil
}