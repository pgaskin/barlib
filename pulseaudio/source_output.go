package pulseaudio

import (
    errors2 "errors"
    "io"
    "math"
    "strings"
)

// SourceOutput contains information about a source output in pulseaudio
type SourceOutput struct {
    Index          uint32
    Name           string
    OwnerModule    uint32
    ClientIndex    uint32
    Source         uint32
    SampleSpec     sampleSpec
    ChannelMap     channelMap
    Cvolume        cvolume
    BufferUsec     uint64
    SourceUsec     uint64
    ResampleMethod string
    Driver         string
    Muted          bool
    PropList       map[string]string
    Corked         bool
    HasVolume      bool
    VolumeWritable bool
    Format         formatInfo
    Client         *Client
}

// ReadFrom deserializes a source output packet from pulseaudio
func (s *SourceOutput) ReadFrom(r io.Reader) (int64, error) {
    err := bread(r,
        uint32Tag, &s.Index,
        stringTag, &s.Name,
        uint32Tag, &s.OwnerModule,
        uint32Tag, &s.ClientIndex,
        uint32Tag, &s.Source,
        &s.SampleSpec,
        &s.ChannelMap,
        usecTag, &s.BufferUsec,
        usecTag, &s.SourceUsec,
        stringTag, &s.ResampleMethod,
        stringTag, &s.Driver,
        &s.PropList,
        &s.Corked,
        &s.Cvolume,
        &s.Muted,
        &s.HasVolume,
        &s.VolumeWritable)
    if err != nil {
        return 0, err
    }
    err = bread(r, &s.Format)
    return 0, nil
}

func (s SourceOutput) SetVolume(volume float32) error {
    _, err := s.Client.request(commandSetSourceOutputVolume, uint32Tag, s.Index, cvolume{uint32(volume * 0xffff)})
    return err
}

func (s SourceOutput) SetMute(b bool) error {
    muteCmd := '0'
    if b {
        muteCmd = '1'
    }
    _, err := s.Client.request(commandSetSourceOutputMute, uint32Tag, s.Index, uint8(muteCmd))
    return err
}

func (s SourceOutput) ToggleMute() error {
    return s.SetMute(!s.Muted)
}

func (s SourceOutput) IsMute() bool {
    return s.Muted
}

func (s SourceOutput) GetVolume() float32 {
    return float32(math.Round(float64(float32(s.Cvolume[0])/0xffff)*100)) / 100
}

// SourceOutputs queries PulseAudio for a list of source outputs and returns an array
func (c *Client) SourceOutputs() ([]SourceOutput, error) {
    b, err := c.request(commandGetSourceOutputInfoList)
    if err != nil {
        return nil, err
    }
    var sourceOutputs []SourceOutput
    for b.Len() > 0 {
        var sourceOutput SourceOutput
        err = bread(b, &sourceOutput)
        if err != nil {
            return nil, err
        }
        sourceOutput.Client = c
        sourceOutputs = append(sourceOutputs, sourceOutput)
    }
    return sourceOutputs, nil
}

func (c *Client) GetSourceOutputByName(name string) (SourceOutput, error) {
    sourceOutputs, err := c.SourceOutputs()
    if err != nil {
        return SourceOutput{}, err
    }
    for _, sourceOutput := range sourceOutputs {
        sourceName, ok := sourceOutput.PropList["application.name"]
        if !ok {
            continue
        }
        if strings.ToLower(sourceName) == strings.ToLower(name) {
            return sourceOutput, nil
        }
    }
    return SourceOutput{}, errors2.New("Could not get source output: " + name)
}
