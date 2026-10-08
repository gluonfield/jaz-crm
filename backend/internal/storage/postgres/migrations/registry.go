package migrations

import "github.com/pressly/goose/v3"

var All = []*goose.Migration{DealFollowups, FollowUps, ChaseFilter, PageIcons, CoreProperties}
