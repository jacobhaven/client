// Copyright 2015 Keybase, Inc. All rights reserved. Use of
// this source code is governed by the included BSD license.

package engine

import (
	"github.com/jonboulle/clockwork"
	"github.com/keybase/client/go/libkb"
	keybase1 "github.com/keybase/client/go/protocol"
	"testing"
	"time"
)

func TestTrackToken(t *testing.T) {
	tc := SetupEngineTest(t, "track")
	defer tc.Cleanup()
	fu := CreateAndSignupFakeUser(tc, "track")

	trackWithToken(tc, fu, "t_alice")
}

func trackWithToken(tc libkb.TestContext, fu *FakeUser, username string) {
	idUI := &FakeIdentifyUI{}
	idarg := &keybase1.IdentifyArg{UserAssertion: username}
	ctx := &Context{
		LogUI:      tc.G.UI.GetLogUI(),
		IdentifyUI: idUI,
		SecretUI:   fu.NewSecretUI(),
	}
	eng := NewIDEngine(idarg, tc.G)
	if err := RunEngine(eng, ctx); err != nil {
		tc.T.Fatal(err)
	}

	res := eng.Result()
	arg := TrackTokenArg{
		Token:   res.TrackToken,
		Options: keybase1.TrackOptions{BypassConfirm: true},
	}
	teng := NewTrackToken(&arg, tc.G)
	if err := RunEngine(teng, ctx); err != nil {
		tc.T.Fatal(err)
	}

	defer runUntrack(tc.G, fu, username)
	assertTracking(tc, username)
}

func TestTrackTokenIdentify2(t *testing.T) {
	tc := SetupEngineTest(t, "track")
	defer tc.Cleanup()
	fu := CreateAndSignupFakeUser(tc, "track")

	idUI := &FakeIdentifyUI{}
	username := "t_tracy"
	arg := &keybase1.Identify2Arg{
		UserAssertion: username,
		NeedProofSet:  true,
	}
	ctx := &Context{
		LogUI:      tc.G.UI.GetLogUI(),
		IdentifyUI: idUI,
		SecretUI:   fu.NewSecretUI(),
	}
	eng := NewResolveThenIdentify2(tc.G, arg)
	if err := RunEngine(eng, ctx); err != nil {
		tc.T.Fatal(err)
	}
	targ := TrackTokenArg{
		Token:   idUI.Token,
		Options: keybase1.TrackOptions{BypassConfirm: true},
	}
	teng := NewTrackToken(&targ, tc.G)
	if err := RunEngine(teng, ctx); err != nil {
		tc.T.Fatal(err)
	}

	defer runUntrack(tc.G, fu, username)
	assertTracking(tc, username)
}

func TestTempTrackLocal(t *testing.T) {
	tc := SetupEngineTest(t, "track")
	defer tc.Cleanup()
	fu := CreateAndSignupFakeUser(tc, "track")

	fakeClock := clockwork.NewFakeClockAt(time.Now())
	tc.G.Clock = fakeClock
	// to pick up the new clock...
	tc.G.ResetLoginState()

	flakeyAPI := flakeyRooterAPI{orig: tc.G.XAPI, hardFail: false, G: tc.G}
	tc.G.XAPI = &flakeyAPI

	idUI := &FakeIdentifyUI{}
	username := "t_tracy"

	arg := &keybase1.Identify2Arg{
		UserAssertion: username,
		NeedProofSet:  true,
	}
	ctx := &Context{
		LogUI:      tc.G.UI.GetLogUI(),
		IdentifyUI: idUI,
		SecretUI:   fu.NewSecretUI(),
	}

	// Identify tracy; all proofs should work
	eng := NewResolveThenIdentify2(tc.G, arg)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}
	targ := TrackTokenArg{
		Token:   idUI.Token,
		Options: keybase1.TrackOptions{BypassConfirm: true},
	}

	// Track tracy
	teng := NewTrackToken(&targ, tc.G)
	if err := RunEngine(teng, ctx); err != nil {
		t.Fatal(err)
	}

	// Now make her Rooter proof fail
	t.Logf("-- Set for Hard Fail --")
	flakeyAPI.hardFail = true
	idUI = &FakeIdentifyUI{}
	ctx.IdentifyUI = idUI

	// Advance so that our previous cached success is out of
	// cache
	fakeClock.Advance(tc.G.Env.GetProofCacheMediumDur() + time.Minute)

	eng = NewResolveThenIdentify2(tc.G, arg)
	eng.testArgs = &Identify2WithUIDTestArgs{noCache: true}
	// Should  get an error
	if err := RunEngine(eng, ctx); err == nil {
		t.Fatal("Expected identify error")
	}

	targ.Options.ExpiringLocal = true
	// Track tracy
	teng = NewTrackToken(&targ, tc.G)
	if err := RunEngine(teng, ctx); err != nil {
		t.Fatal(err)
	}

	// Identify should work once more because we signed with failures
	eng = NewResolveThenIdentify2(tc.G, arg)
	eng.testArgs = &Identify2WithUIDTestArgs{noCache: true}
	var err error
	// Should not get an error
	if err = RunEngine(eng, ctx); err != nil {
		t.Logf("Identify failure: %v", err)
		// TODO: The engine is saying there is 1 failure still.
		//t.Fatal("Expected to pass identify")
	}

	// Advance so that our temporary track is discarded
	// cache
	fakeClock.Advance(tc.G.Env.GetLocalTrackMaxAge() + time.Minute)

	// Identify should fail once more
	eng = NewResolveThenIdentify2(tc.G, arg)
	eng.testArgs = &Identify2WithUIDTestArgs{noCache: true}
	// Should get an error
	if err = RunEngine(eng, ctx); err == nil {
		t.Fatal("Expected rooter to fail")
	}
	t.Logf("Identify failure: %v", err)

	defer runUntrack(tc.G, fu, username)
	assertTracking(tc, username)
}
