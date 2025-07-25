package lorawan

// CN470SubBand CN470子频段定义
type CN470SubBand int

const (
    CN470SubBand1A CN470SubBand = iota // 470.3-471.9 MHz (Ch 0-7)
    CN470SubBand1B                      // 472.3-473.9 MHz (Ch 8-15)
    CN470SubBand2A                      // 474.3-475.9 MHz (Ch 16-23)
    CN470SubBand2B                      // 476.3-477.9 MHz (Ch 24-31)
    CN470SubBand3A                      // 478.3-479.9 MHz (Ch 32-39)
    CN470SubBand3B                      // 480.3-481.9 MHz (Ch 40-47)
    CN470SubBand4A                      // 482.3-483.9 MHz (Ch 48-55)
    CN470SubBand4B                      // 484.3-485.9 MHz (Ch 56-63)
    CN470SubBand5A                      // 486.3-487.9 MHz (Ch 64-71)
    CN470SubBand5B                      // 488.3-489.9 MHz (Ch 72-79)
    CN470SubBand6A                      // 490.3-491.9 MHz (Ch 80-87)
    CN470SubBand6B                      // 492.3-493.9 MHz (Ch 88-95)
)

// CN470GetUplinkFrequency 获取CN470上行频率
func CN470GetUplinkFrequency(channel int) uint32 {
    if channel < 0 || channel > 95 {
        return 0
    }
    return uint32(470300000 + channel*200000)
}

// CN470GetDownlinkFrequency 获取CN470下行频率
func CN470GetDownlinkFrequency(channel int) uint32 {
    if channel < 0 || channel > 47 {
        return 0
    }
    return uint32(500300000 + channel*200000)
}

// CN470GetDownlinkChannelForUplink 根据上行信道获取对应的下行信道
func CN470GetDownlinkChannelForUplink(uplinkChannel int) int {
    // CN470: 下行信道 = 上行信道 % 48
    return uplinkChannel % 48
}

// CN470ConfigureChannels 配置CN470信道（每次激活8个）
func CN470ConfigureChannels(channelMask uint16, subBand CN470SubBand) []Channel {
    var channels []Channel
    baseChannel := int(subBand) * 8
    
    for i := 0; i < 8; i++ {
        if channelMask&(1<<uint(i)) != 0 {
            ch := baseChannel + i
            if ch < 96 {
                channels = append(channels, Channel{
                    Frequency: CN470GetUplinkFrequency(ch),
                    MinDR:     0,
                    MaxDR:     5,
                })
            }
        }
    }
    
    return channels
}
