import { createRouter, createWebHistory } from 'vue-router';
import LoginView from '@/views/LoginView.vue';
import SignupView from '@/views/SignupView.vue';
import DashboardView from '@/views/DashboardView.vue';
import AccountsView from '@/views/AccountsView.vue';
import AccountsManageView from '@/views/AccountsManageView.vue';
import SpendingView from '@/views/SpendingView.vue';
import RulesView from '@/views/RulesView.vue';
import InboxView from '@/views/InboxView.vue';
import CategoriesView from '@/views/CategoriesView.vue';
import InvestmentsView from '@/views/InvestmentsView.vue';
import InvestmentDetailView from '@/views/InvestmentDetailView.vue';
import ProfileView from '@/views/ProfileView.vue';
import UiKitView from '@/views/UiKitView.vue';

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: LoginView },
    { path: '/signup', component: SignupView },
    {
      path: '/',
      component: () => import('@/components/AppLayout.vue'),
      children: [
        { path: '', redirect: '/dashboard' },
        { path: 'dashboard', name: 'dashboard', component: DashboardView },
        { path: 'accounts', name: 'accounts', component: AccountsView },
        { path: 'accounts/manage', name: 'accounts-manage', component: AccountsManageView },
        { path: 'accounts/spending', name: 'spending', component: SpendingView },
        { path: 'accounts/rules', name: 'rules', component: RulesView },
        { path: 'inbox', name: 'inbox', component: InboxView },
        { path: 'accounts/categories', name: 'categories', component: CategoriesView },
        { path: 'investments', name: 'investments', component: InvestmentsView },
        { path: 'investments/:id', name: 'investment-detail', component: InvestmentDetailView },
        { path: 'settings', name: 'settings', component: ProfileView },
        // Not in the nav: the primitives, in every state, for looking at.
        { path: 'ui-kit', name: 'ui-kit', component: UiKitView }
      ]
    },
    { path: '/:pathMatch(.*)*', redirect: '/' }
  ]
});

export default router;
