package timer

import "time"

const (
	SecMs      = 1000
	MinMs      = 60 * SecMs
	HourMs     = 60 * MinMs
	DayMs      = 24 * HourMs
	WeekMs     = 7 * DayMs
	MonthMs    = 30 * DayMs
	YearMs     = 365 * DayMs
	TenYearsMs = 10 * YearMs
)

// 时钟
type Clock struct {
	Location   *time.Location // 时区
	offset     int64          // 时间偏移量,单位毫秒，用于调时间，比如将服务器时间往前调1个小时，offset=3600000
	zoneOffset int64          // 时区偏移毫秒数，用于定义服务器的时区偏移
}

func NewClock() *Clock {
	return &Clock{
		Location:   time.UTC,
		offset:     0,
		zoneOffset: 0,
	}
}

func (c *Clock) SetLocation(location *time.Location) {
	c.Location = location
}

func (c *Clock) SetOffset(offset int64) {
	c.offset = offset
}

func (c *Clock) SetZoneOffset(zoneOffset int64) {
	c.zoneOffset = zoneOffset
}

func (c *Clock) Now() time.Time {
	now := time.Now()
	if c.offset > 0 {
		now = now.Add(time.Duration(c.offset))
	}
	return now.In(c.Location)
}

func (c *Clock) NowUnix() int64 {
	return c.Now().Unix()
}

func (c *Clock) NowUnixMilli() int64 {
	return c.Now().UnixMilli()
}

// 根据毫秒时间戳获取星期
func (c *Clock) GetWeekday(timestamp int64) int64 {
	ws := (timestamp + c.zoneOffset) % WeekMs
	d := ws / DayMs
	return (d + 4) % 7
}

func (c *Clock) IsSameHour(t1, t2 time.Time) bool {
	if t1.Year() != t2.Year() || t1.Month() != t2.Month() || t1.Day() != t2.Day() {
		return false
	}
	return t1.Hour() == t2.Hour()
}

func (c *Clock) IsSameDay(t1, t2 time.Time) bool {
	if t1.Year() != t2.Year() || t1.Month() != t2.Month() {
		return false
	}
	return t1.Day() == t2.Day()
}

func (c *Clock) IsSameWeek(t1, t2 time.Time) bool {
	year1, week1 := t1.ISOWeek()
	year2, week2 := t2.ISOWeek()
	return year1 == year2 && week1 == week2
}
