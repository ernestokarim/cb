<?php

use Illuminate\Auth\UserInterface;

class User extends BaseModel implements UserInterface {

	public function getAuthIdentifier() {
		return $this->getKey();
	}

	public function getAuthPassword() {
		return $this->password;
	}

}
