import { User, newUser } from "./user";

export default () => {
  const user: User = newUser("John");
  console.log(user);
};
