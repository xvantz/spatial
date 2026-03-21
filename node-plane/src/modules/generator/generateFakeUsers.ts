import { IUser } from "../../types/user";

const randomFloat = (min: number, max: number): number => {
  return Math.random() * (max - min) + min;
};

const WORLD = {
  SIZE_X: 3500,
  SIZE_Y: 3500,
  SIZE_Z: 1000,
};

export const generateFakeUsers = (length: number): IUser[] => {
  return Array.from({ length }, (_, index) => ({
    id: index + 1,
    position: {
      x: randomFloat(-WORLD.SIZE_X, WORLD.SIZE_X),
      y: randomFloat(-WORLD.SIZE_Y, WORLD.SIZE_Y),
      z: randomFloat(-WORLD.SIZE_Z, WORLD.SIZE_Z),
    },
  }));
};
