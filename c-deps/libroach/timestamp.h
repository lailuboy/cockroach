// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License included
// in the file licenses/BSL.txt and at www.mariadb.com/bsl11.
//
// Change Date: 2022-10-01
//
// On the date above, in accordance with the Business Source License, use
// of this software will be governed by the Apache License, Version 2.0,
// included in the file licenses/APL.txt and at
// https://www.apache.org/licenses/LICENSE-2.0

#pragma once

#include <libroach.h>
#include "protos/storage/engine/enginepb/mvcc.pb.h"

namespace cockroach {

const DBTimestamp kZeroTimestamp = {0, 0};

DBTimestamp ToDBTimestamp(const cockroach::util::hlc::LegacyTimestamp& timestamp) {
  return DBTimestamp{timestamp.wall_time(), timestamp.logical()};
}

DBTimestamp PrevTimestamp(DBTimestamp ts) {
  if (ts.logical > 0) {
    --ts.logical;
  } else if (ts.wall_time == 0) {
    fprintf(stderr, "no previous time for zero timestamp\n");
    abort();
  } else {
    --ts.wall_time;
    ts.logical = std::numeric_limits<int32_t>::max();
  }
  return ts;
}

inline bool operator==(const DBTimestamp& a, const DBTimestamp& b) {
  return a.wall_time == b.wall_time && a.logical == b.logical;
}

inline bool operator!=(const DBTimestamp& a, const DBTimestamp& b) { return !(a == b); }

inline bool operator<(const DBTimestamp& a, const DBTimestamp& b) {
  return a.wall_time < b.wall_time || (a.wall_time == b.wall_time && a.logical < b.logical);
}

inline bool operator>(const DBTimestamp& a, const DBTimestamp& b) { return b < a; }

inline bool operator<=(const DBTimestamp& a, const DBTimestamp& b) { return !(b < a); }

inline bool operator>=(const DBTimestamp& a, const DBTimestamp& b) { return b <= a; }

}  // namespace cockroach
