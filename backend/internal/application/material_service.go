package application

import "github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"

type MaterialService struct{ Repo material.Repository }

func (s MaterialService) Save(a material.Asset) error {
	return s.Repo.Save(a)
}

func (s MaterialService) List(k material.Kind) []material.Asset {
	return s.Repo.List(k)
}

func (s MaterialService) Get(id string) (material.Asset, bool) {
	return s.Repo.Get(id)
}

func (s MaterialService) Delete(id string) error {
	return s.Repo.Delete(id)
}
