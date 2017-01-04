package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
)

func withTorrentContext(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var ih metainfo.Hash
		err := ih.FromHexString(q.Get("ih"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ref := torrentRefs.NewRef(ih)
		tc := r.Context().Value(torrentClientContextKey).(*torrent.Client)
		t, new := tc.AddTorrentInfoHash(ih)
		ref.SetCloser(t.Drop)
		defer time.AfterFunc(time.Minute, ref.Release)
		mi := cachedMetaInfo(ih)
		if mi != nil {
			t.AddTrackers(mi.AnnounceList)
			t.SetInfoBytes(mi.InfoBytes)
		}
		if new {
			go saveTorrentWhenGotInfo(t)
		}
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), torrentContextKey, ref)))
	})
}

func saveTorrentWhenGotInfo(t *torrent.Torrent) {
	select {
	case <-t.Closed():
		return
	case <-t.GotInfo():
	}
	err := saveTorrentFile(t)
	if err != nil {
		log.Printf("error saving torrent file: %s", err)
	}
}

func cachedMetaInfo(infoHash metainfo.Hash) *metainfo.MetaInfo {
	mi, err := metainfo.LoadFromFile(fmt.Sprintf("torrents/%s.torrent", infoHash.HexString()))
	if err == nil && mi.HashInfoBytes() == infoHash {
		return mi
	}
	return nil
}
