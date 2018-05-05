package holochain

//------------------------------------------------------------
// CloseBundle

type APIFnCloseBundle struct {
	commit bool
}

func (a *APIFnCloseBundle) Name() string {
	return "bundleClose"
}

func (a *APIFnCloseBundle) Args() []Arg {
	return []Arg{{Name: "commit", Type: BoolArg}}
}

func (a *APIFnCloseBundle) Call(h *Holochain) (response interface{}, err error) {

	bundle := h.Chain().BundleStarted()
	if bundle == nil {
		err = ErrBundleNotStarted
		return
	}

	isCancel := !a.commit
	// if this is a cancel call all the bundleCancel routines
	if isCancel {
		for _, zome := range h.nucleus.dna.Zomes {
			var r Ribosome
			r, _, err = h.MakeRibosome(zome.Name)
			if err != nil {
				continue
			}
			var result string
			result, err = r.BundleCanceled(BundleCancelReasonUserCancel)
			if err != nil {
				Debugf("error in %s.bundleCanceled():%v", zome.Name, err)
				continue
			}
			if result == BundleCancelResponseCommit {
				Debugf("%s.bundleCanceled() overrode cancel", zome.Name)
				err = nil
				return
			}
		}
	}
	err = h.Chain().CloseBundle(a.commit)
	if err == nil {
		// if there wasn't an error closing the bundle share all the commits
		for _, a := range bundle.sharing {
			_, def, err := h.GetEntryDef(a.GetHeader().Type)
			if err != nil {
				h.dht.dlog.Logf("Error getting entry def in close bundle:%v", err)
				err = nil
			} else {
				err = a.Share(h, def)
			}
		}
	}
	return
}
