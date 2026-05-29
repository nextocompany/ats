package scoring

// JD is the scoring view of a position's Master JD. The pipeline maps a
// positions.Position into this struct, keeping scoring decoupled from the
// positions repository.
type JD struct {
	MinEducationLevel   int
	MinExperienceMonths int
	Keywords            []string
}
