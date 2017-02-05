/**
 * Created by yen-chiehchen on 2/4/17.
 */

import { NgModule }             from '@angular/core';
import { RouterModule, Routes } from '@angular/router';

import { UserComponent } from './user.component'
import { DashboardComponent} from './dashboard.component'
import { KidComponent } from './kid.component'
import { ActivityComponent } from './activity.component'

const routes: Routes = [
    { path: 'dashboard', component: DashboardComponent },
    { path: 'user', component: UserComponent },
    { path: 'device', component: KidComponent },
    { path: 'activity/:kidId', component: ActivityComponent }
];

@NgModule({
    imports: [RouterModule.forRoot(routes, { useHash: true })],
    exports: [RouterModule]
})

export class Routing{}