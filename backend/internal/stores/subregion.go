// Package stores owns store reference data and the province→subregion mapping
// used by branch assignment. The mapping + province centroids are real data and
// survive the swap to the real Storelist.csv.
package stores

import "strings"

// The 13 subregions (PRP F04).
const (
	SubBKKEast      = "BKK East"
	SubBKKWest1     = "BKK West 1"
	SubBKKWest2     = "BKK West 2"
	SubCentralNorth = "Central North"
	SubCentralWest  = "Central West"
	SubUpperNorth   = "Upper North"
	SubLowerNorth   = "Lower North"
	SubUpperNE      = "Upper Northeast"
	SubLowerEastNE  = "Lower East Northeast"
	SubLowerWestNE  = "Lower West Northeast"
	SubEast         = "East"
	SubUpperSouth   = "Upper South"
	SubLowerSouth   = "Lower South"
)

// ProvinceInfo carries a province's subregion and approximate centroid.
type ProvinceInfo struct {
	Subregion string
	Lat       float64
	Lng       float64
}

// provinceMap maps Thai province names to subregion + centroid. Coordinates are
// approximate centroids — precise enough for nearest-store ranking.
var provinceMap = map[string]ProvinceInfo{
	// Bangkok & vicinity
	"กรุงเทพมหานคร": {SubBKKEast, 13.7563, 100.5018},
	"นนทบุรี":       {SubBKKWest1, 13.8591, 100.5217},
	"ปทุมธานี":      {SubBKKWest1, 14.0208, 100.5250},
	"สมุทรปราการ":   {SubBKKEast, 13.5991, 100.5998},
	"สมุทรสาคร":     {SubBKKWest2, 13.5475, 100.2746},
	"นครปฐม":        {SubBKKWest2, 13.8199, 100.0621},
	"สมุทรสงคราม":   {SubBKKWest2, 13.4098, 100.0021},
	// Central North
	"พระนครศรีอยุธยา": {SubCentralNorth, 14.3692, 100.5877},
	"อ่างทอง":         {SubCentralNorth, 14.5896, 100.4550},
	"ลพบุรี":          {SubCentralNorth, 14.7995, 100.6534},
	"สิงห์บุรี":       {SubCentralNorth, 14.8907, 100.3967},
	"ชัยนาท":          {SubCentralNorth, 15.1851, 100.1251},
	"สระบุรี":         {SubCentralNorth, 14.5289, 100.9101},
	"นครสวรรค์":       {SubCentralNorth, 15.7047, 100.1372},
	"อุทัยธานี":       {SubCentralNorth, 15.3835, 100.0246},
	// Central West
	"กาญจนบุรี":       {SubCentralWest, 14.0227, 99.5328},
	"ราชบุรี":         {SubCentralWest, 13.5283, 99.8134},
	"เพชรบุรี":        {SubCentralWest, 13.1119, 99.9399},
	"ประจวบคีรีขันธ์": {SubCentralWest, 11.8126, 99.7957},
	"สุพรรณบุรี":      {SubCentralWest, 14.4745, 100.1177},
	// Upper North
	"เชียงใหม่":  {SubUpperNorth, 18.7883, 98.9853},
	"เชียงราย":   {SubUpperNorth, 19.9105, 99.8406},
	"ลำพูน":      {SubUpperNorth, 18.5743, 99.0087},
	"ลำปาง":      {SubUpperNorth, 18.2888, 99.4909},
	"แม่ฮ่องสอน": {SubUpperNorth, 19.3020, 97.9654},
	"พะเยา":      {SubUpperNorth, 19.1664, 99.9020},
	"แพร่":       {SubUpperNorth, 18.1445, 100.1405},
	"น่าน":       {SubUpperNorth, 18.7756, 100.7730},
	// Lower North
	"พิษณุโลก":  {SubLowerNorth, 16.8211, 100.2659},
	"สุโขทัย":   {SubLowerNorth, 17.0070, 99.8265},
	"ตาก":       {SubLowerNorth, 16.8840, 99.1258},
	"กำแพงเพชร": {SubLowerNorth, 16.4828, 99.5226},
	"พิจิตร":    {SubLowerNorth, 16.4429, 100.3487},
	"เพชรบูรณ์": {SubLowerNorth, 16.4190, 101.1591},
	"อุตรดิตถ์": {SubLowerNorth, 17.6200, 100.0993},
	// Upper Northeast
	"อุดรธานี":    {SubUpperNE, 17.4138, 102.7870},
	"หนองคาย":     {SubUpperNE, 17.8783, 102.7420},
	"เลย":         {SubUpperNE, 17.4860, 101.7223},
	"หนองบัวลำภู": {SubUpperNE, 17.2218, 102.4260},
	"บึงกาฬ":      {SubUpperNE, 18.3609, 103.6466},
	"สกลนคร":      {SubUpperNE, 17.1545, 104.1348},
	"นครพนม":      {SubUpperNE, 17.3910, 104.7690},
	"มุกดาหาร":    {SubUpperNE, 16.5420, 104.7210},
	"กาฬสินธุ์":   {SubUpperNE, 16.4314, 103.5059},
	// Lower East Northeast
	"อุบลราชธานี": {SubLowerEastNE, 15.2440, 104.8473},
	"ศรีสะเกษ":    {SubLowerEastNE, 15.1186, 104.3220},
	"ยโสธร":       {SubLowerEastNE, 15.7921, 104.1452},
	"อำนาจเจริญ":  {SubLowerEastNE, 15.8657, 104.6258},
	"ร้อยเอ็ด":    {SubLowerEastNE, 16.0538, 103.6520},
	"มหาสารคาม":   {SubLowerEastNE, 16.1850, 103.3007},
	// Lower West Northeast
	"นครราชสีมา": {SubLowerWestNE, 14.9799, 102.0978},
	"บุรีรัมย์":  {SubLowerWestNE, 14.9930, 103.1029},
	"สุรินทร์":   {SubLowerWestNE, 14.8818, 103.4936},
	"ชัยภูมิ":    {SubLowerWestNE, 15.8068, 102.0317},
	"ขอนแก่น":    {SubLowerWestNE, 16.4419, 102.8360},
	// East
	"ชลบุรี":     {SubEast, 13.3611, 100.9847},
	"ระยอง":      {SubEast, 12.6814, 101.2780},
	"จันทบุรี":   {SubEast, 12.6113, 102.1035},
	"ตราด":       {SubEast, 12.2436, 102.5151},
	"ฉะเชิงเทรา": {SubEast, 13.6904, 101.0779},
	"ปราจีนบุรี": {SubEast, 14.0479, 101.3686},
	"นครนายก":    {SubEast, 14.2069, 101.2130},
	"สระแก้ว":    {SubEast, 13.8240, 102.0645},
	// Upper South
	"ชุมพร":         {SubUpperSouth, 10.4930, 99.1800},
	"ระนอง":         {SubUpperSouth, 9.9529, 98.6085},
	"สุราษฎร์ธานี":  {SubUpperSouth, 9.1382, 99.3215},
	"นครศรีธรรมราช": {SubUpperSouth, 8.4304, 99.9631},
	"กระบี่":        {SubUpperSouth, 8.0863, 98.9063},
	"พังงา":         {SubUpperSouth, 8.4509, 98.5256},
	"ภูเก็ต":        {SubUpperSouth, 7.8804, 98.3923},
	// Lower South
	"สงขลา":    {SubLowerSouth, 7.1897, 100.5951},
	"ตรัง":     {SubLowerSouth, 7.5563, 99.6114},
	"พัทลุง":   {SubLowerSouth, 7.6167, 100.0742},
	"สตูล":     {SubLowerSouth, 6.6238, 100.0674},
	"ปัตตานี":  {SubLowerSouth, 6.8692, 101.2502},
	"ยะลา":     {SubLowerSouth, 6.5413, 101.2803},
	"นราธิวาส": {SubLowerSouth, 6.4254, 101.8253},
}

// ResolveSubregion returns the subregion for a Thai province name, or "" when
// unknown (the caller routes unknown provinces to the talent pool).
func ResolveSubregion(province string) string {
	if info, ok := provinceMap[strings.TrimSpace(province)]; ok {
		return info.Subregion
	}
	return ""
}

// ProvinceCentroid returns the approximate centroid for a province and whether
// it is known.
func ProvinceCentroid(province string) (lat, lng float64, ok bool) {
	if info, ok := provinceMap[strings.TrimSpace(province)]; ok {
		return info.Lat, info.Lng, true
	}
	return 0, 0, false
}
