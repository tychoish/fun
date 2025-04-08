package internal

import "sync"

func Lock(mtx *sync.Mutex) *sync.Mutex { mtx.Lock(); return mtx }
func With(mtx *sync.Mutex)             { mtx.Unlock() }

func LockL(mtx sync.Locker) sync.Locker { mtx.Lock(); return mtx }
func WithL(mtx sync.Locker)             { mtx.Unlock() }
