package xlinkclient

import "regexp"

type SystemStats interface {
	Ident() string
	PtpSync() bool
	PtpSyncLocal() bool
	Ptp() bool
	OSUpTime() int64
	CPUTemp() int
	SysTemp() int
}

type EthStats interface {
	Ident() string
	RX() float32
	TX() float32
}

type DecoderStats interface {
	Ident() string
	RTT() float32
	UpTime() int64
	StatsTime() int64
	FromCloud() int64
	FromP2P() int64
	Dropped() int64
	Resent() int64
	ResentDropped() int64
	VideoDTotal() int64
	VideoDDrop() int64
	VideoDCorr() int64
	VideoDMissing() int64
	VideoRMissing() int64
	VideoOutFps() float32
	RXmbps() float32
	TXmbps() float32
}

type EncoderStats interface {
	Ident() string
	UpTime() int64
	StatsTime() int64
	VideoInFps() float32
}

type Stats struct {
	SysId string `json:"sysid"`
	Data  struct {
		Local []*StatsLocal `json:"local"`
	} `json:"data"`
}

func (s Stats) Id() string {
	return s.SysId
}

var systemReg = regexp.MustCompile(`^X\d[A-Z]\d+$`)
var encReg = regexp.MustCompile(`^X\d[A-Z]\d+-E\d+$`)
var decReg = regexp.MustCompile(`^X\d[A-Z]\d+-D\d+$`)
var ethReg = regexp.MustCompile(`^eth\d+$`)

func (s Stats) SystemStats() SystemStats {
	for _, stat := range s.Data.Local {
		matched := systemReg.MatchString(stat.Id)
		if matched {
			return stat
		}
	}
	return nil
}

func (s Stats) EthStats() []EthStats {
	stats := make([]EthStats, 0)
	for _, stat := range s.Data.Local {
		matched := ethReg.MatchString(stat.Id)
		if matched {
			stats = append(stats, stat)
		}
	}
	return stats
}

func (s Stats) EncoderStats() []EncoderStats {
	stats := make([]EncoderStats, 0)
	for _, stat := range s.Data.Local {
		matched := encReg.MatchString(stat.Id)
		if matched {
			stats = append(stats, stat)
		}
	}
	return stats
}

func (s Stats) DecoderStats() []DecoderStats {
	stats := make([]DecoderStats, 0)
	for _, stat := range s.Data.Local {
		matched := decReg.MatchString(stat.Id)
		if matched {
			stats = append(stats, stat)
		}
	}
	return stats
}

type StatsLocal struct {
	Id   string `json:"id"`
	Type int    `json:"type"`
	Data struct {
		PtpSync      bool    `json:"ptpSync"`
		PtpSyncLocal bool    `json:"ptpSyncLocal"`
		Ptp          bool    `json:"ptp"`
		OSUpTime     int64   `json:"osUpTime"`
		CPUTemp      int     `json:"cpuTemp"`
		SysTemp      int     `json:"sysTemp"`
		EthRX        float32 `json:"rx"`
		EthTX        float32 `json:"tx"`
		DEXLink      struct {
			RTT        float32 `json:"rtt"`
			Cloud      int64   `json:"cloud"`
			P2P        int64   `json:"p2p"`
			Drop       int64   `json:"drop"`
			Resent     int64   `json:"resent"`
			ResentDrop int64   `json:"resentDrop"`
		} `json:"xLink"`
		DEvDstats struct {
			Total        int64 `json:"total"`
			Disp         int64 `json:"disp"`
			Drop         int64 `json:"drop"`
			Corr         int64 `json:"corr"`
			Missing      int64 `json:"missing"`
			FECOK        int64 `json:"fecok"`
			FECCorrected int64 `json:"fecCorrected"`
			FECNOK       int64 `json:"fecnok"`
		} `json:"vDstats"`
		DEaDstats struct {
			Total  int64 `json:"total"`
			Drop   int64 `json:"drop"`
			Miss   int64 `json:"miss"`
			UnSync int64 `json:"unsync"`
		} `json:"aDstats"`
		DEvRStats struct {
			Missing      int64 `json:"total"`
			Last         int64 `json:"last"`
			LateDrop     int64 `json:"lateDrop"`
			LateDropLast int64 `json:"lateDropLast"`
		} `json:"vRstats"`
		DEMBPS struct {
			TX float32 `json:"tx"`
			RX float32 `json:"rx"`
		} `json:"mbps"`
		DEvOutFps float32 `json:"vOutFps"`
		UPTime    int64   `json:"upTime"`
		StatsTime int64   `json:"statsTime"`
		ENvInFps  float32 `json:"vInFps"`
	} `json:"data"`
}

func (l StatsLocal) Ident() string {
	return l.Id
}
func (l StatsLocal) PtpSync() bool {
	return l.Data.PtpSync
}
func (l StatsLocal) PtpSyncLocal() bool {
	return l.Data.PtpSyncLocal
}
func (l StatsLocal) Ptp() bool {
	return l.Data.Ptp
}
func (l StatsLocal) OSUpTime() int64 {
	return l.Data.OSUpTime
}
func (l StatsLocal) CPUTemp() int {
	return l.Data.CPUTemp
}
func (l StatsLocal) SysTemp() int {
	return l.Data.SysTemp
}
func (l StatsLocal) RTT() float32 {
	return l.Data.DEXLink.RTT
}
func (l StatsLocal) UpTime() int64 {
	return l.Data.UPTime
}
func (l StatsLocal) StatsTime() int64 {
	return l.Data.StatsTime
}
func (l StatsLocal) FromCloud() int64 {
	return l.Data.DEXLink.Cloud
}
func (l StatsLocal) FromP2P() int64 {
	return l.Data.DEXLink.P2P
}
func (l StatsLocal) Dropped() int64 {
	return l.Data.DEXLink.Drop
}
func (l StatsLocal) Resent() int64 {
	return l.Data.DEXLink.Resent
}
func (l StatsLocal) ResentDropped() int64 {
	return l.Data.DEXLink.ResentDrop
}
func (l StatsLocal) VideoDTotal() int64 {
	return l.Data.DEvDstats.Total
}
func (l StatsLocal) VideoDDrop() int64 {
	return l.Data.DEvDstats.Drop
}
func (l StatsLocal) VideoDCorr() int64 {
	return l.Data.DEvDstats.Corr
}
func (l StatsLocal) VideoDMissing() int64 {
	return l.Data.DEvDstats.Missing
}
func (l StatsLocal) VideoRMissing() int64 {
	return l.Data.DEvRStats.Missing
}
func (l StatsLocal) VideoOutFps() float32 {
	return l.Data.DEvOutFps
}
func (l StatsLocal) RXmbps() float32 {
	return l.Data.DEMBPS.RX
}
func (l StatsLocal) TXmbps() float32 {
	return l.Data.DEMBPS.TX
}
func (l StatsLocal) RX() float32 {
	return l.Data.EthRX
}
func (l StatsLocal) TX() float32 {
	return l.Data.EthTX
}
func (l StatsLocal) VideoInFps() float32 {
	return l.Data.ENvInFps
}
